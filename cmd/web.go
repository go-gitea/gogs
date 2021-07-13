// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" // Used for debugging if enabled and a web server is running
	"os"
	"strings"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/routers/install"

	context2 "github.com/gorilla/context"
	"github.com/urfave/cli"
	ini "gopkg.in/ini.v1"
)

// CmdWeb represents the available web sub-command.
var CmdWeb = cli.Command{
	Name:  "web",
	Usage: "Start Gitea web server",
	Description: `Gitea web server is the only thing you need to run,
and it takes care of all the other things for you`,
	Action: runWeb,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "port, p",
			Value: "3000",
			Usage: "Temporary port number to prevent conflict",
		},
		cli.StringFlag{
			Name:  "install-port",
			Value: "3000",
			Usage: "Temporary port number to run the install page on to prevent conflict",
		},
		cli.StringFlag{
			Name:  "pid, P",
			Value: setting.PIDFile,
			Usage: "Custom pid file path",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Only display Fatal logging errors until logging is set-up",
		},
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "Set initial logging to TRACE level until logging is properly set-up",
		},
	},
}

func runHTTPRedirector() {
	source := fmt.Sprintf("%s:%s", setting.HTTPAddr, setting.PortToRedirect)
	dest := strings.TrimSuffix(setting.AppURL, "/")
	log.Info("Redirecting: %s to %s", source, dest)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := dest + r.URL.Path
		if len(r.URL.RawQuery) > 0 {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	})

	var err = runHTTP("tcp", source, "HTTP Redirector", context2.ClearHandler(handler))

	if err != nil {
		log.Fatal("Failed to start port redirection: %v", err)
	}
}

func runWeb(ctx *cli.Context) error {
	if ctx.Bool("verbose") {
		_ = log.DelLogger("console")
		log.NewLogger(0, "console", "console", fmt.Sprintf(`{"level": "trace", "colorize": %t, "stacktraceLevel": "none"}`, log.CanColorStdout))
	} else if ctx.Bool("quiet") {
		_ = log.DelLogger("console")
		log.NewLogger(0, "console", "console", fmt.Sprintf(`{"level": "fatal", "colorize": %t, "stacktraceLevel": "none"}`, log.CanColorStdout))
	}

	managerCtx, cancel := context.WithCancel(context.Background())
	graceful.InitManager(managerCtx)
	defer cancel()

	if os.Getppid() > 1 && len(os.Getenv("LISTEN_FDS")) > 0 {
		log.Info("Restarting Gitea on PID: %d from parent PID: %d", os.Getpid(), os.Getppid())
	} else {
		log.Info("Starting Gitea on PID: %d", os.Getpid())
	}

	// Set pid file setting
	if ctx.IsSet("pid") {
		setting.PIDFile = ctx.String("pid")
		setting.WritePIDFile = true
	}

	// Perform pre-initialization
	needsInstall := install.PreloadSettings(graceful.GetManager().HammerContext())
	if needsInstall {
		// Flag for port number in case first time run conflict
		if ctx.IsSet("port") {
			if err := setPort(ctx.String("port")); err != nil {
				return err
			}
		}
		if ctx.IsSet("install-port") {
			if err := setPort(ctx.String("install-port")); err != nil {
				return err
			}
		}
		c := install.Routes()
		err := listen(c, false)
		select {
		case <-graceful.GetManager().IsShutdown():
			<-graceful.GetManager().Done()
			log.Info("PID: %d Gitea Web Finished", os.Getpid())
			log.Close()
			return err
		default:
		}
	} else {
		NoInstallListener()
	}

	if setting.EnablePprof {
		go func() {
			log.Info("Starting pprof server on localhost:6060")
			log.Info("%v", http.ListenAndServe("localhost:6060", nil))
		}()
	}

	log.Info("Global init")
	// Perform global initialization
	routers.GlobalInit(graceful.GetManager().HammerContext())

	// Override the provided port number within the configuration
	if ctx.IsSet("port") {
		if err := setPort(ctx.String("port")); err != nil {
			return err
		}
	}

	// Set up Chi routes
	c := routers.NormalRoutes()
	err := listen(c, true)
	<-graceful.GetManager().Done()
	log.Info("PID: %d Gitea Web Finished", os.Getpid())
	log.Close()
	return err
}

func setPort(port string) error {
	setting.AppURL = strings.Replace(setting.AppURL, setting.HTTPPort, port, 1)
	setting.HTTPPort = port

	switch setting.Protocol {
	case setting.UnixSocket:
	case setting.FCGI:
	case setting.FCGIUnix:
	default:
		defaultLocalURL := string(setting.Protocol) + "://"
		if setting.HTTPAddr == "0.0.0.0" {
			defaultLocalURL += "localhost"
		} else {
			defaultLocalURL += setting.HTTPAddr
		}
		defaultLocalURL += ":" + setting.HTTPPort + "/"

		// Save LOCAL_ROOT_URL if port changed
		setting.CreateOrAppendToCustomConf(func(cfg *ini.File) {
			cfg.Section("server").Key("LOCAL_ROOT_URL").SetValue(defaultLocalURL)
		})
	}
	return nil
}

func listen(m http.Handler, handleRedirector bool) error {
	listenAddr := setting.HTTPAddr
	if setting.Protocol != setting.UnixSocket && setting.Protocol != setting.FCGIUnix {
		listenAddr = net.JoinHostPort(listenAddr, setting.HTTPPort)
	}
	log.Info("Listen: %v://%s%s", setting.Protocol, listenAddr, setting.AppSubURL)

	if setting.LFS.StartServer {
		log.Info("LFS server enabled")
	}

	var err error
	switch setting.Protocol {
	case setting.HTTP:
		if handleRedirector {
			NoHTTPRedirector()
		}
		err = runHTTP("tcp", listenAddr, "Web", context2.ClearHandler(m))
	case setting.HTTPS:
		if setting.EnableLetsEncrypt {
			err = runLetsEncrypt(listenAddr, setting.Domain, setting.LetsEncryptDirectory, setting.LetsEncryptEmail, context2.ClearHandler(m))
			break
		}
		if handleRedirector {
			if setting.RedirectOtherPort {
				go runHTTPRedirector()
			} else {
				NoHTTPRedirector()
			}
		}
		err = runHTTPS("tcp", listenAddr, "Web", setting.CertFile, setting.KeyFile, context2.ClearHandler(m))
	case setting.FCGI:
		if handleRedirector {
			NoHTTPRedirector()
		}
		err = runFCGI("tcp", listenAddr, "FCGI Web", context2.ClearHandler(m))
	case setting.UnixSocket:
		if handleRedirector {
			NoHTTPRedirector()
		}
		err = runHTTP("unix", listenAddr, "Web", context2.ClearHandler(m))
	case setting.FCGIUnix:
		if handleRedirector {
			NoHTTPRedirector()
		}
		err = runFCGI("unix", listenAddr, "Web", context2.ClearHandler(m))
	default:
		log.Fatal("Invalid protocol: %s", setting.Protocol)
	}

	if err != nil {
		log.Critical("Failed to start server: %v", err)
	}
	log.Info("HTTP Listener: %s Closed", listenAddr)
	return err
}
