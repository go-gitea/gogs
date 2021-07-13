// Copyright 2015 Matthew Holt
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package certmagic

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/mholt/acmez/acme"
	"go.uber.org/zap"
	"golang.org/x/crypto/ocsp"
)

// maintainAssets is a permanently-blocking function
// that loops indefinitely and, on a regular schedule, checks
// certificates for expiration and initiates a renewal of certs
// that are expiring soon. It also updates OCSP stapling. It
// should only be called once per cache. Panics are recovered,
// and if panicCount < 10, the function is called recursively,
// incrementing panicCount each time. Initial invocation should
// start panicCount at 0.
func (certCache *Cache) maintainAssets(panicCount int) {
	log := loggerNamed(certCache.logger, "maintenance")
	if log != nil {
		log = log.With(zap.String("cache", fmt.Sprintf("%p", certCache)))
	}

	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, stackTraceBufferSize)
			buf = buf[:runtime.Stack(buf, false)]
			if log != nil {
				log.Error("panic", zap.Any("error", err), zap.ByteString("stack", buf))
			}
			if panicCount < 10 {
				certCache.maintainAssets(panicCount + 1)
			}
		}
	}()

	renewalTicker := time.NewTicker(certCache.options.RenewCheckInterval)
	ocspTicker := time.NewTicker(certCache.options.OCSPCheckInterval)

	if log != nil {
		log.Info("started background certificate maintenance")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		select {
		case <-renewalTicker.C:
			err := certCache.RenewManagedCertificates(ctx)
			if err != nil && log != nil {
				log.Error("renewing managed certificates", zap.Error(err))
			}
		case <-ocspTicker.C:
			certCache.updateOCSPStaples(ctx)
		case <-certCache.stopChan:
			renewalTicker.Stop()
			ocspTicker.Stop()
			// TODO: stop any in-progress maintenance operations and clear locks we made (this might be done now with our use of context)
			if log != nil {
				log.Info("stopped background certificate maintenance")
			}
			close(certCache.doneChan)
			return
		}
	}
}

// RenewManagedCertificates renews managed certificates,
// including ones loaded on-demand. Note that this is done
// automatically on a regular basis; normally you will not
// need to call this. This method assumes non-interactive
// mode (i.e. operating in the background).
func (certCache *Cache) RenewManagedCertificates(ctx context.Context) error {
	log := loggerNamed(certCache.logger, "maintenance")

	// configs will hold a map of certificate name to the config
	// to use when managing that certificate
	configs := make(map[string]*Config)

	// we use the queues for a very important reason: to do any and all
	// operations that could require an exclusive write lock outside
	// of the read lock! otherwise we get a deadlock, yikes. in other
	// words, our first iteration through the certificate cache does NOT
	// perform any operations--only queues them--so that more fine-grained
	// write locks may be obtained during the actual operations.
	var renewQueue, reloadQueue, deleteQueue []Certificate

	certCache.mu.RLock()
	for certKey, cert := range certCache.cache {
		if !cert.managed {
			continue
		}

		// the list of names on this cert should never be empty... programmer error?
		if cert.Names == nil || len(cert.Names) == 0 {
			if log != nil {
				log.Warn("certificate has no names; removing from cache", zap.String("cert_key", certKey))
			}
			deleteQueue = append(deleteQueue, cert)
			continue
		}

		// get the config associated with this certificate
		cfg, err := certCache.getConfig(cert)
		if err != nil {
			if log != nil {
				log.Error("unable to get configuration to manage certificate; unable to renew",
					zap.Strings("identifiers", cert.Names),
					zap.Error(err))
			}
			continue
		}
		if cfg == nil {
			// this is bad if this happens, probably a programmer error (oops)
			if log != nil {
				log.Error("no configuration associated with certificate; unable to manage",
					zap.Strings("identifiers", cert.Names))
			}
			continue
		}
		if cfg.OnDemand != nil {
			continue
		}

		// if time is up or expires soon, we need to try to renew it
		if cert.NeedsRenewal(cfg) {
			configs[cert.Names[0]] = cfg

			// see if the certificate in storage has already been renewed, possibly by another
			// instance that didn't coordinate with this one; if so, just load it (this
			// might happen if another instance already renewed it - kinda sloppy but checking disk
			// first is a simple way to possibly drastically reduce rate limit problems)
			storedCertExpiring, err := cfg.managedCertInStorageExpiresSoon(cert)
			if err != nil {
				// hmm, weird, but not a big deal, maybe it was deleted or something
				if log != nil {
					log.Warn("error while checking if stored certificate is also expiring soon",
						zap.Strings("identifiers", cert.Names),
						zap.Error(err))
				}
			} else if !storedCertExpiring {
				// if the certificate is NOT expiring soon and there was no error, then we
				// are good to just reload the certificate from storage instead of repeating
				// a likely-unnecessary renewal procedure
				reloadQueue = append(reloadQueue, cert)
				continue
			}

			// the certificate in storage has not been renewed yet, so we will do it
			// NOTE: It is super-important to note that the TLS-ALPN challenge requires
			// a write lock on the cache in order to complete its challenge, so it is extra
			// vital that this renew operation does not happen inside our read lock!
			renewQueue = append(renewQueue, cert)
		}
	}
	certCache.mu.RUnlock()

	// Reload certificates that merely need to be updated in memory
	for _, oldCert := range reloadQueue {
		timeLeft := oldCert.Leaf.NotAfter.Sub(time.Now().UTC())
		if log != nil {
			log.Info("certificate expires soon, but is already renewed in storage; reloading stored certificate",
				zap.Strings("identifiers", oldCert.Names),
				zap.Duration("remaining", timeLeft))
		}

		cfg := configs[oldCert.Names[0]]

		// crucially, this happens OUTSIDE a lock on the certCache
		err := cfg.reloadManagedCertificate(oldCert)
		if err != nil {
			if log != nil {
				log.Error("loading renewed certificate",
					zap.Strings("identifiers", oldCert.Names),
					zap.Error(err))
			}
			continue
		}
	}

	// Renewal queue
	for _, oldCert := range renewQueue {
		cfg := configs[oldCert.Names[0]]
		err := certCache.queueRenewalTask(ctx, oldCert, cfg)
		if err != nil {
			if log != nil {
				log.Error("queueing renewal task",
					zap.Strings("identifiers", oldCert.Names),
					zap.Error(err))
			}
			continue
		}
	}

	// Deletion queue
	certCache.mu.Lock()
	for _, cert := range deleteQueue {
		certCache.removeCertificate(cert)
	}
	certCache.mu.Unlock()

	return nil
}

func (certCache *Cache) queueRenewalTask(ctx context.Context, oldCert Certificate, cfg *Config) error {
	log := loggerNamed(certCache.logger, "maintenance")

	timeLeft := oldCert.Leaf.NotAfter.Sub(time.Now().UTC())
	if log != nil {
		log.Info("certificate expires soon; queuing for renewal",
			zap.Strings("identifiers", oldCert.Names),
			zap.Duration("remaining", timeLeft))
	}

	// Get the name which we should use to renew this certificate;
	// we only support managing certificates with one name per cert,
	// so this should be easy.
	renewName := oldCert.Names[0]

	// queue up this renewal job (is a no-op if already active or queued)
	jm.Submit(cfg.Logger, "renew_"+renewName, func() error {
		timeLeft := oldCert.Leaf.NotAfter.Sub(time.Now().UTC())
		if log != nil {
			log.Info("attempting certificate renewal",
				zap.Strings("identifiers", oldCert.Names),
				zap.Duration("remaining", timeLeft))
		}

		// perform renewal - crucially, this happens OUTSIDE a lock on certCache
		err := cfg.RenewCertAsync(ctx, renewName, false)
		if err != nil {
			if cfg.OnDemand != nil {
				// loaded dynamically, remove dynamically
				certCache.mu.Lock()
				certCache.removeCertificate(oldCert)
				certCache.mu.Unlock()
			}
			return fmt.Errorf("%v %v", oldCert.Names, err)
		}

		// successful renewal, so update in-memory cache by loading
		// renewed certificate so it will be used with handshakes
		err = cfg.reloadManagedCertificate(oldCert)
		if err != nil {
			return ErrNoRetry{fmt.Errorf("%v %v", oldCert.Names, err)}
		}
		return nil
	})

	return nil
}

// updateOCSPStaples updates the OCSP stapling in all
// eligible, cached certificates.
//
// OCSP maintenance strives to abide the relevant points on
// Ryan Sleevi's recommendations for good OCSP support:
// https://gist.github.com/sleevi/5efe9ef98961ecfb4da8
func (certCache *Cache) updateOCSPStaples(ctx context.Context) {
	logger := loggerNamed(certCache.logger, "maintenance")

	// temporary structures to store updates or tasks
	// so that we can keep our locks short-lived
	type ocspUpdate struct {
		rawBytes []byte
		parsed   *ocsp.Response
	}
	type updateQueueEntry struct {
		cert           Certificate
		certHash       string
		lastNextUpdate time.Time
	}
	type renewQueueEntry struct {
		oldCert  Certificate
		ocspResp *ocsp.Response
	}
	updated := make(map[string]ocspUpdate)
	var updateQueue []updateQueueEntry // certs that need a refreshed staple
	var renewQueue []renewQueueEntry   // certs that need to be renewed (due to revocation)
	configs := make(map[string]*Config)

	// obtain brief read lock during our scan to see which staples need updating
	certCache.mu.RLock()
	for certHash, cert := range certCache.cache {
		// no point in updating OCSP for expired or "synthetic" certificates
		if cert.Leaf == nil || cert.Expired() {
			continue
		}
		var lastNextUpdate time.Time
		if cert.ocsp != nil {
			lastNextUpdate = cert.ocsp.NextUpdate
			if freshOCSP(cert.ocsp) {
				continue // no need to update staple if ours is still fresh
			}
		}
		updateQueue = append(updateQueue, updateQueueEntry{cert, certHash, lastNextUpdate})
	}
	certCache.mu.RUnlock()

	// perform updates outside of any lock on certCache
	for _, qe := range updateQueue {
		cert := qe.cert
		certHash := qe.certHash
		lastNextUpdate := qe.lastNextUpdate

		cfg, err := certCache.getConfig(cert)
		if err != nil {
			if logger != nil {
				logger.Error("unable to refresh OCSP staple because getting automation config for certificate failed",
					zap.Strings("identifiers", cert.Names),
					zap.Error(err))
			}
			continue
		}
		if cfg == nil {
			// this is bad if this happens, probably a programmer error (oops)
			if logger != nil {
				logger.Error("no configuration associated with certificate; unable to manage OCSP staples",
					zap.Strings("identifiers", cert.Names))
			}
			continue
		}

		ocspResp, err := stapleOCSP(cfg.OCSP, cfg.Storage, &cert, nil)
		if err != nil || ocspResp == nil {
			if cert.ocsp != nil {
				// if there was no staple before, that's fine; otherwise we should log the error
				if logger != nil {
					logger.Error("stapling OCSP",
						zap.Strings("identifiers", cert.Names),
						zap.Error(err))
				}
			}
			continue
		}

		// By this point, we've obtained the latest OCSP response.
		// If there was no staple before, or if the response is updated, make
		// sure we apply the update to all names on the certificate.
		if cert.ocsp != nil && (lastNextUpdate.IsZero() || lastNextUpdate != cert.ocsp.NextUpdate) {
			if logger != nil {
				logger.Info("advancing OCSP staple",
					zap.Strings("identifiers", cert.Names),
					zap.Time("from", lastNextUpdate),
					zap.Time("to", cert.ocsp.NextUpdate))
			}
			updated[certHash] = ocspUpdate{rawBytes: cert.Certificate.OCSPStaple, parsed: cert.ocsp}
		}

		// If a managed certificate was revoked, we should attempt to replace it with a new one.
		if cert.managed && ocspResp.Status == ocsp.Revoked && len(cert.Names) > 0 {
			renewQueue = append(renewQueue, renewQueueEntry{
				oldCert:  cert,
				ocspResp: ocspResp,
			})
			configs[cert.Names[0]] = cfg
		}
	}

	// These write locks should be brief since we have all the info we need now.
	for certKey, update := range updated {
		certCache.mu.Lock()
		cert := certCache.cache[certKey]
		cert.ocsp = update.parsed
		cert.Certificate.OCSPStaple = update.rawBytes
		certCache.cache[certKey] = cert
		certCache.mu.Unlock()
	}

	// We attempt to replace any certificates that were revoked.
	// Crucially, this happens OUTSIDE a lock on the certCache.
	for _, renew := range renewQueue {
		if logger != nil {
			logger.Warn("OCSP status for managed certificate is REVOKED; attempting to replace with new certificate",
				zap.Strings("identifiers", renew.oldCert.Names),
				zap.Time("expiration", renew.oldCert.Leaf.NotAfter))
		}

		renewName := renew.oldCert.Names[0]
		cfg := configs[renewName]

		// if revoked for key compromise, we can't be sure whether the storage of
		// the key is still safe; however, we KNOW the old key is not safe, and we
		// can only hope by the time of revocation that storage has been secured;
		// key management is not something we want to get into, but in this case
		// it seems prudent to replace the key - and since renewal requires reuse
		// of a prior key, we can't do a "renew" to replace the cert if we need a
		// new key, so we'll have to do an obtain instead
		var obtainInsteadOfRenew bool
		if renew.ocspResp.RevocationReason == acme.ReasonKeyCompromise {
			err := cfg.moveCompromisedPrivateKey(renew.oldCert, logger)
			if err != nil && logger != nil {
				logger.Error("could not remove compromised private key from use",
					zap.Strings("identifiers", renew.oldCert.Names),
					zap.String("issuer", renew.oldCert.issuerKey),
					zap.Error(err))
			}
			obtainInsteadOfRenew = true
		}

		var err error
		if obtainInsteadOfRenew {
			err = cfg.ObtainCertAsync(ctx, renewName)
		} else {
			// notice that we force renewal; otherwise, it might see that the
			// certificate isn't close to expiring and return, but we really
			// need a replacement certificate! see issue #4191
			err = cfg.RenewCertAsync(ctx, renewName, true)
		}
		if err != nil {
			// probably better to not serve a revoked certificate at all
			if logger != nil {
				logger.Error("unable to obtain new to certificate after OCSP status of REVOKED; removing from cache",
					zap.Strings("identifiers", renew.oldCert.Names),
					zap.Error(err))
			}
			certCache.mu.Lock()
			certCache.removeCertificate(renew.oldCert)
			certCache.mu.Unlock()
			continue
		}
		err = cfg.reloadManagedCertificate(renew.oldCert)
		if err != nil {
			if logger != nil {
				logger.Error("after obtaining new certificate due to OCSP status of REVOKED",
					zap.Strings("identifiers", renew.oldCert.Names),
					zap.Error(err))
			}
			continue
		}
	}
}

// CleanStorageOptions specifies how to clean up a storage unit.
type CleanStorageOptions struct {
	OCSPStaples            bool
	ExpiredCerts           bool
	ExpiredCertGracePeriod time.Duration
}

// CleanStorage removes assets which are no longer useful,
// according to opts.
func CleanStorage(ctx context.Context, storage Storage, opts CleanStorageOptions) {
	if opts.OCSPStaples {
		err := deleteOldOCSPStaples(ctx, storage)
		if err != nil {
			log.Printf("[ERROR] Deleting old OCSP staples: %v", err)
		}
	}
	if opts.ExpiredCerts {
		err := deleteExpiredCerts(ctx, storage, opts.ExpiredCertGracePeriod)
		if err != nil {
			log.Printf("[ERROR] Deleting expired certificates: %v", err)
		}
	}
	// TODO: delete stale locks?
}

func deleteOldOCSPStaples(ctx context.Context, storage Storage) error {
	ocspKeys, err := storage.List(prefixOCSP, false)
	if err != nil {
		// maybe just hasn't been created yet; no big deal
		return nil
	}
	for _, key := range ocspKeys {
		// if context was cancelled, quit early; otherwise proceed
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		ocspBytes, err := storage.Load(key)
		if err != nil {
			log.Printf("[ERROR] While deleting old OCSP staples, unable to load staple file: %v", err)
			continue
		}
		resp, err := ocsp.ParseResponse(ocspBytes, nil)
		if err != nil {
			// contents are invalid; delete it
			err = storage.Delete(key)
			if err != nil {
				log.Printf("[ERROR] Purging corrupt staple file %s: %v", key, err)
			}
			continue
		}
		if time.Now().After(resp.NextUpdate) {
			// response has expired; delete it
			err = storage.Delete(key)
			if err != nil {
				log.Printf("[ERROR] Purging expired staple file %s: %v", key, err)
			}
		}
	}
	return nil
}

func deleteExpiredCerts(ctx context.Context, storage Storage, gracePeriod time.Duration) error {
	issuerKeys, err := storage.List(prefixCerts, false)
	if err != nil {
		// maybe just hasn't been created yet; no big deal
		return nil
	}

	for _, issuerKey := range issuerKeys {
		siteKeys, err := storage.List(issuerKey, false)
		if err != nil {
			log.Printf("[ERROR] Listing contents of %s: %v", issuerKey, err)
			continue
		}

		for _, siteKey := range siteKeys {
			// if context was cancelled, quit early; otherwise proceed
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			siteAssets, err := storage.List(siteKey, false)
			if err != nil {
				log.Printf("[ERROR] Listing contents of %s: %v", siteKey, err)
				continue
			}

			for _, assetKey := range siteAssets {
				if path.Ext(assetKey) != ".crt" {
					continue
				}

				certFile, err := storage.Load(assetKey)
				if err != nil {
					return fmt.Errorf("loading certificate file %s: %v", assetKey, err)
				}
				block, _ := pem.Decode(certFile)
				if block == nil || block.Type != "CERTIFICATE" {
					return fmt.Errorf("certificate file %s does not contain PEM-encoded certificate", assetKey)
				}
				cert, err := x509.ParseCertificate(block.Bytes)
				if err != nil {
					return fmt.Errorf("certificate file %s is malformed; error parsing PEM: %v", assetKey, err)
				}

				if expiredTime := time.Since(cert.NotAfter); expiredTime >= gracePeriod {
					log.Printf("[INFO] Certificate %s expired %s ago; cleaning up", assetKey, expiredTime)
					baseName := strings.TrimSuffix(assetKey, ".crt")
					for _, relatedAsset := range []string{
						assetKey,
						baseName + ".key",
						baseName + ".json",
					} {
						log.Printf("[INFO] Deleting %s because resource expired", relatedAsset)
						err := storage.Delete(relatedAsset)
						if err != nil {
							log.Printf("[ERROR] Cleaning up asset related to expired certificate for %s: %s: %v",
								baseName, relatedAsset, err)
						}
					}
				}
			}

			// update listing; if folder is empty, delete it
			siteAssets, err = storage.List(siteKey, false)
			if err != nil {
				continue
			}
			if len(siteAssets) == 0 {
				log.Printf("[INFO] Deleting %s because key is empty", siteKey)
				err := storage.Delete(siteKey)
				if err != nil {
					return fmt.Errorf("deleting empty site folder %s: %v", siteKey, err)
				}
			}
		}
	}
	return nil
}

// moveCompromisedPrivateKey moves the private key for cert to a ".compromised" file
// by copying the data to the new file, then deleting the old one.
func (cfg *Config) moveCompromisedPrivateKey(cert Certificate, logger *zap.Logger) error {
	privKeyStorageKey := StorageKeys.SitePrivateKey(cert.issuerKey, cert.Names[0])

	privKeyPEM, err := cfg.Storage.Load(privKeyStorageKey)
	if err != nil {
		return err
	}

	compromisedPrivKeyStorageKey := privKeyStorageKey + ".compromised"
	err = cfg.Storage.Store(compromisedPrivKeyStorageKey, privKeyPEM)
	if err != nil {
		// better safe than sorry: as a last resort, try deleting the key so it won't be reused
		cfg.Storage.Delete(privKeyStorageKey)
		return err
	}

	err = cfg.Storage.Delete(privKeyStorageKey)
	if err != nil {
		return err
	}

	logger.Info("removed certificate's compromised private key from use",
		zap.String("storage_path", compromisedPrivKeyStorageKey),
		zap.Strings("identifiers", cert.Names),
		zap.String("issuer", cert.issuerKey))

	return nil
}

const (
	// DefaultRenewCheckInterval is how often to check certificates for expiration.
	// Scans are very lightweight, so this can be semi-frequent. This default should
	// be smaller than <Minimum Cert Lifetime>*DefaultRenewalWindowRatio/3, which
	// gives certificates plenty of chance to be renewed on time.
	DefaultRenewCheckInterval = 10 * time.Minute

	// DefaultRenewalWindowRatio is how much of a certificate's lifetime becomes the
	// renewal window. The renewal window is the span of time at the end of the
	// certificate's validity period in which it should be renewed. A default value
	// of ~1/3 is pretty safe and recommended for most certificates.
	DefaultRenewalWindowRatio = 1.0 / 3.0

	// DefaultOCSPCheckInterval is how often to check if OCSP stapling needs updating.
	DefaultOCSPCheckInterval = 1 * time.Hour
)
