// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// WorkerPool represent a dynamically growable worker pool for a
// provided handler function. They have an internal channel which
// they use to detect if there is a block and will grow and shrink in
// response to demand as per configuration.
type WorkerPool struct {
	lock               sync.Mutex
	baseCtx            context.Context
	baseCtxCancel      context.CancelFunc
	paused             chan struct{}
	resumed            chan struct{}
	cond               *sync.Cond
	qid                int64
	maxNumberOfWorkers int
	numberOfWorkers    int
	batchLength        int
	handle             HandlerFunc
	dataChan           chan Data
	blockTimeout       time.Duration
	boostTimeout       time.Duration
	boostWorkers       int
	numInQueue         int64
}

var _ Flushable = &WorkerPool{}
var _ ManagedPool = &WorkerPool{}

// WorkerPoolConfiguration is the basic configuration for a WorkerPool
type WorkerPoolConfiguration struct {
	QueueLength  int
	BatchLength  int
	BlockTimeout time.Duration
	BoostTimeout time.Duration
	BoostWorkers int
	MaxWorkers   int
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(handle HandlerFunc, config WorkerPoolConfiguration) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	dataChan := make(chan Data, config.QueueLength)
	resumed := make(chan struct{})
	close(resumed)
	pool := &WorkerPool{
		baseCtx:            ctx,
		baseCtxCancel:      cancel,
		batchLength:        config.BatchLength,
		dataChan:           dataChan,
		resumed:            resumed,
		paused:             make(chan struct{}),
		handle:             handle,
		blockTimeout:       config.BlockTimeout,
		boostTimeout:       config.BoostTimeout,
		boostWorkers:       config.BoostWorkers,
		maxNumberOfWorkers: config.MaxWorkers,
	}

	return pool
}

// Push pushes the data to the internal channel
func (p *WorkerPool) Push(data Data) {
	atomic.AddInt64(&p.numInQueue, 1)
	p.lock.Lock()
	select {
	case <-p.paused:
		p.lock.Unlock()
		p.dataChan <- data
		return
	default:
	}

	if p.blockTimeout > 0 && p.boostTimeout > 0 && (p.numberOfWorkers <= p.maxNumberOfWorkers || p.maxNumberOfWorkers < 0) {
		if p.numberOfWorkers == 0 {
			p.zeroBoost()
		} else {
			p.lock.Unlock()
		}
		p.pushBoost(data)
	} else {
		p.lock.Unlock()
		p.dataChan <- data
	}
}

func (p *WorkerPool) zeroBoost() {
	ctx, cancel := context.WithTimeout(p.baseCtx, p.boostTimeout)
	mq := GetManager().GetManagedQueue(p.qid)
	boost := p.boostWorkers
	if (boost+p.numberOfWorkers) > p.maxNumberOfWorkers && p.maxNumberOfWorkers >= 0 {
		boost = p.maxNumberOfWorkers - p.numberOfWorkers
	}
	if mq != nil {
		log.Warn("WorkerPool: %d (for %s) has zero workers - adding %d temporary workers for %s", p.qid, mq.Name, boost, p.boostTimeout)

		start := time.Now()
		pid := mq.RegisterWorkers(boost, start, true, start.Add(p.boostTimeout), cancel, false)
		cancel = func() {
			mq.RemoveWorkers(pid)
		}
	} else {
		log.Warn("WorkerPool: %d has zero workers - adding %d temporary workers for %s", p.qid, p.boostWorkers, p.boostTimeout)
	}
	p.lock.Unlock()
	p.addWorkers(ctx, cancel, boost)
}

func (p *WorkerPool) pushBoost(data Data) {
	select {
	case p.dataChan <- data:
	default:
		p.lock.Lock()
		if p.blockTimeout <= 0 {
			p.lock.Unlock()
			p.dataChan <- data
			return
		}
		ourTimeout := p.blockTimeout
		timer := time.NewTimer(p.blockTimeout)
		p.lock.Unlock()
		select {
		case p.dataChan <- data:
			util.StopTimer(timer)
		case <-timer.C:
			p.lock.Lock()
			if p.blockTimeout > ourTimeout || (p.numberOfWorkers > p.maxNumberOfWorkers && p.maxNumberOfWorkers >= 0) {
				p.lock.Unlock()
				p.dataChan <- data
				return
			}
			p.blockTimeout *= 2
			boostCtx, boostCtxCancel := context.WithCancel(p.baseCtx)
			mq := GetManager().GetManagedQueue(p.qid)
			boost := p.boostWorkers
			if (boost+p.numberOfWorkers) > p.maxNumberOfWorkers && p.maxNumberOfWorkers >= 0 {
				boost = p.maxNumberOfWorkers - p.numberOfWorkers
			}
			if mq != nil {
				log.Warn("WorkerPool: %d (for %s) Channel blocked for %v - adding %d temporary workers for %s, block timeout now %v", p.qid, mq.Name, ourTimeout, boost, p.boostTimeout, p.blockTimeout)

				start := time.Now()
				pid := mq.RegisterWorkers(boost, start, true, start.Add(p.boostTimeout), boostCtxCancel, false)
				go func() {
					<-boostCtx.Done()
					mq.RemoveWorkers(pid)
					boostCtxCancel()
				}()
			} else {
				log.Warn("WorkerPool: %d Channel blocked for %v - adding %d temporary workers for %s, block timeout now %v", p.qid, ourTimeout, p.boostWorkers, p.boostTimeout, p.blockTimeout)
			}
			go func() {
				<-time.After(p.boostTimeout)
				boostCtxCancel()
				p.lock.Lock()
				p.blockTimeout /= 2
				p.lock.Unlock()
			}()
			p.lock.Unlock()
			p.addWorkers(boostCtx, boostCtxCancel, boost)
			p.dataChan <- data
		}
	}
}

// NumberOfWorkers returns the number of current workers in the pool
func (p *WorkerPool) NumberOfWorkers() int {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.numberOfWorkers
}

// MaxNumberOfWorkers returns the maximum number of workers automatically added to the pool
func (p *WorkerPool) MaxNumberOfWorkers() int {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.maxNumberOfWorkers
}

// BoostWorkers returns the number of workers for a boost
func (p *WorkerPool) BoostWorkers() int {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.boostWorkers
}

// BoostTimeout returns the timeout of the next boost
func (p *WorkerPool) BoostTimeout() time.Duration {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.boostTimeout
}

// BlockTimeout returns the timeout til the next boost
func (p *WorkerPool) BlockTimeout() time.Duration {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.blockTimeout
}

// SetPoolSettings sets the setable boost values
func (p *WorkerPool) SetPoolSettings(maxNumberOfWorkers, boostWorkers int, timeout time.Duration) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.maxNumberOfWorkers = maxNumberOfWorkers
	p.boostWorkers = boostWorkers
	p.boostTimeout = timeout
}

// SetMaxNumberOfWorkers sets the maximum number of workers automatically added to the pool
// Changing this number will not change the number of current workers but will change the limit
// for future additions
func (p *WorkerPool) SetMaxNumberOfWorkers(newMax int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.maxNumberOfWorkers = newMax
}

func (p *WorkerPool) commonRegisterWorkers(number int, timeout time.Duration, isFlusher bool) (context.Context, context.CancelFunc) {
	var ctx context.Context
	var cancel context.CancelFunc
	start := time.Now()
	end := start
	hasTimeout := false
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(p.baseCtx, timeout)
		end = start.Add(timeout)
		hasTimeout = true
	} else {
		ctx, cancel = context.WithCancel(p.baseCtx)
	}

	mq := GetManager().GetManagedQueue(p.qid)
	if mq != nil {
		pid := mq.RegisterWorkers(number, start, hasTimeout, end, cancel, isFlusher)
		log.Trace("WorkerPool: %d (for %s) adding %d workers with group id: %d", p.qid, mq.Name, number, pid)
		return ctx, func() {
			mq.RemoveWorkers(pid)
		}
	}
	log.Trace("WorkerPool: %d adding %d workers (no group id)", p.qid, number)

	return ctx, cancel
}

// AddWorkers adds workers to the pool - this allows the number of workers to go above the limit
func (p *WorkerPool) AddWorkers(number int, timeout time.Duration) context.CancelFunc {
	ctx, cancel := p.commonRegisterWorkers(number, timeout, false)
	p.addWorkers(ctx, cancel, number)
	return cancel
}

// addWorkers adds workers to the pool
func (p *WorkerPool) addWorkers(ctx context.Context, cancel context.CancelFunc, number int) {
	for i := 0; i < number; i++ {
		p.lock.Lock()
		if p.cond == nil {
			p.cond = sync.NewCond(&p.lock)
		}
		p.numberOfWorkers++
		p.lock.Unlock()
		go func() {
			p.doWork(ctx)

			p.lock.Lock()
			p.numberOfWorkers--
			if p.numberOfWorkers == 0 {
				p.cond.Broadcast()
				cancel()
			} else if p.numberOfWorkers < 0 {
				// numberOfWorkers can't go negative but...
				log.Warn("Number of Workers < 0 for QID %d - this shouldn't happen", p.qid)
				p.numberOfWorkers = 0
				p.cond.Broadcast()
				cancel()
			}
			p.lock.Unlock()
		}()
	}
}

// Wait for WorkerPool to finish
func (p *WorkerPool) Wait() {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.cond == nil {
		p.cond = sync.NewCond(&p.lock)
	}
	if p.numberOfWorkers <= 0 {
		return
	}
	p.cond.Wait()
}

// IsPaused returns if the pool is paused
func (p *WorkerPool) IsPaused() bool {
	p.lock.Lock()
	defer p.lock.Unlock()
	select {
	case <-p.paused:
		return true
	default:
		return false
	}
}

// IsPausedIsResumed returns if the pool is paused and a channel that is closed when it is resumed
func (p *WorkerPool) IsPausedIsResumed() (<-chan struct{}, <-chan struct{}) {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.paused, p.resumed
}

// Pause pauses the WorkerPool
func (p *WorkerPool) Pause() {
	p.lock.Lock()
	defer p.lock.Unlock()
	select {
	case <-p.paused:
	default:
		p.resumed = make(chan struct{})
		close(p.paused)
	}
}

// Resume resumes the WorkerPool
func (p *WorkerPool) Resume() {
	p.lock.Lock()
	defer p.lock.Unlock()
	select {
	case <-p.resumed:
	default:
		p.paused = make(chan struct{})
		close(p.resumed)
	}
}

// CleanUp will drain the remaining contents of the channel
// This should be called after AddWorkers context is closed
func (p *WorkerPool) CleanUp(ctx context.Context) {
	log.Trace("WorkerPool: %d CleanUp", p.qid)
	close(p.dataChan)
	for data := range p.dataChan {
		if unhandled := p.handle(data); unhandled != nil {
			if unhandled != nil {
				log.Error("Unhandled Data in clean-up of queue %d", p.qid)
			}
		}

		atomic.AddInt64(&p.numInQueue, -1)
		select {
		case <-ctx.Done():
			log.Warn("WorkerPool: %d Cleanup context closed before finishing clean-up", p.qid)
			return
		default:
		}
	}
	log.Trace("WorkerPool: %d CleanUp Done", p.qid)
}

// Flush flushes the channel with a timeout - the Flush worker will be registered as a flush worker with the manager
func (p *WorkerPool) Flush(timeout time.Duration) error {
	ctx, cancel := p.commonRegisterWorkers(1, timeout, true)
	defer cancel()
	return p.FlushWithContext(ctx)
}

// IsEmpty returns if true if the worker queue is empty
func (p *WorkerPool) IsEmpty() bool {
	return atomic.LoadInt64(&p.numInQueue) == 0
}

// FlushWithContext is very similar to CleanUp but it will return as soon as the dataChan is empty
// NB: The worker will not be registered with the manager.
func (p *WorkerPool) FlushWithContext(ctx context.Context) error {
	log.Trace("WorkerPool: %d Flush", p.qid)
	for {
		select {
		case data := <-p.dataChan:
			if unhandled := p.handle(data); unhandled != nil {
				log.Error("Unhandled Data whilst flushing queue %d", p.qid)
			}
			atomic.AddInt64(&p.numInQueue, -1)
		case <-p.baseCtx.Done():
			return p.baseCtx.Err()
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
}

func (p *WorkerPool) doWork(ctx context.Context) {
	delay := time.Millisecond * 300

	timer := time.NewTimer(0)
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	var data = make([]Data, 0, p.batchLength)
	paused, _ := p.IsPausedIsResumed()
	for {
		select {
		case <-paused:
			log.Trace("Worker for Queue %d Pausing", p.qid)
			if len(data) > 0 {
				log.Trace("Handling: %d data, %v", len(data), data)
				if unhandled := p.handle(data...); unhandled != nil {
					log.Error("Unhandled Data in queue %d", p.qid)
				}
				atomic.AddInt64(&p.numInQueue, -1*int64(len(data)))
			}
			_, resumed := p.IsPausedIsResumed()
			select {
			case <-resumed:
				paused, _ = p.IsPausedIsResumed()
				log.Trace("Worker for Queue %d Resuming", p.qid)
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			case <-ctx.Done():
				log.Trace("Worker shutting down")
				return
			}
		default:
		}
		select {
		case <-paused:
			// go back around
		case <-ctx.Done():
			if len(data) > 0 {
				log.Trace("Handling: %d data, %v", len(data), data)
				if unhandled := p.handle(data...); unhandled != nil {
					log.Error("Unhandled Data in queue %d", p.qid)
				}
				atomic.AddInt64(&p.numInQueue, -1*int64(len(data)))
			}
			log.Trace("Worker shutting down")
			return
		case datum, ok := <-p.dataChan:
			if !ok {
				// the dataChan has been closed - we should finish up:
				if len(data) > 0 {
					log.Trace("Handling: %d data, %v", len(data), data)
					if unhandled := p.handle(data...); unhandled != nil {
						log.Error("Unhandled Data in queue %d", p.qid)
					}
					atomic.AddInt64(&p.numInQueue, -1*int64(len(data)))
				}
				log.Trace("Worker shutting down")
				return
			}
			data = append(data, datum)
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			if len(data) >= p.batchLength {
				log.Trace("Handling: %d data, %v", len(data), data)
				if unhandled := p.handle(data...); unhandled != nil {
					log.Error("Unhandled Data in queue %d", p.qid)
				}
				atomic.AddInt64(&p.numInQueue, -1*int64(len(data)))
				data = make([]Data, 0, p.batchLength)
			} else {
				timer.Reset(delay)
			}
		case <-timer.C:
			delay = time.Millisecond * 100
			if len(data) > 0 {
				log.Trace("Handling: %d data, %v", len(data), data)
				if unhandled := p.handle(data...); unhandled != nil {
					log.Error("Unhandled Data in queue %d", p.qid)
				}
				atomic.AddInt64(&p.numInQueue, -1*int64(len(data)))
				data = make([]Data, 0, p.batchLength)
			}
		}
	}
}
