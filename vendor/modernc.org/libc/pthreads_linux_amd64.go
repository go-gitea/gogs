// Copyright 2020 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux darwin

package libc // import "modernc.org/libc"

import (
	"sync"

	"modernc.org/libc/errno"
	"modernc.org/libc/pthread"
)

var (
	pthreadConds     = map[uintptr]*pthreadCond{}
	pthreadCondsMu   sync.Mutex
	pthreadMutexes   = map[uintptr]*pthreadMutex{}
	pthreadMutexesMu sync.Mutex
)

// pthread_mutex_t
type pthreadMutex struct {
	sync.Mutex
}

// pthread_cond_t
type pthreadCond struct {
	sync.Cond
}

// int pthread_create(pthread_t *thread, const pthread_attr_t *attr, void *(*start_routine) (void *), void *arg);
func Xpthread_create(tls *TLS, thread, attr, start_routine, arg uintptr) int32 {
	panic(todo(""))
}

// int pthread_detach(pthread_t thread);
func Xpthread_detach(tls *TLS, thread pthread.Pthread_t) int32 {
	panic(todo(""))
}

// int pthread_mutex_lock(pthread_mutex_t *mutex);
func Xpthread_mutex_lock(tls *TLS, mutex uintptr) int32 {
	pthreadMutexesMu.Lock()

	defer pthreadMutexesMu.Unlock()

	if x := pthreadMutexes[mutex]; x != nil {
		x.Lock()
		return 0
	}

	return errno.EINVAL
}

// int pthread_cond_signal(pthread_cond_t *cond);
func Xpthread_cond_signal(tls *TLS, cond uintptr) int32 {
	pthreadCondsMu.Lock()

	defer pthreadCondsMu.Unlock()

	if x := pthreadConds[cond]; x != nil {
		x.Signal()
		return 0
	}

	return errno.EINVAL
}

// int pthread_mutex_unlock(pthread_mutex_t *mutex);
func Xpthread_mutex_unlock(tls *TLS, mutex uintptr) int32 {
	pthreadMutexesMu.Lock()

	defer pthreadMutexesMu.Unlock()

	if x := pthreadMutexes[mutex]; x != nil {
		x.Unlock()
		return 0
	}

	return errno.EINVAL
}

// The pthread_mutex_init() function shall initialize the mutex referenced by
// mutex with attributes specified by attr. If attr is NULL, the default mutex
// attributes are used; the effect shall be the same as passing the address of
// a default mutex attributes object. Upon successful initialization, the state
// of the mutex becomes initialized and unlocked.
//
// If successful, the pthread_mutex_destroy() and pthread_mutex_init()
// functions shall return zero; otherwise, an error number shall be returned to
// indicate the error.
//
// int pthread_mutex_init(pthread_mutex_t *restrict mutex, const pthread_mutexattr_t *restrict attr);
func Xpthread_mutex_init(tls *TLS, mutex, attr uintptr) int32 {
	if attr != 0 {
		panic(todo(""))
	}

	pthreadMutexesMu.Lock()

	defer pthreadMutexesMu.Unlock()

	pthreadMutexes[mutex] = &pthreadMutex{}
	return 0
}

// The pthread_cond_init() function shall initialize the condition variable
// referenced by cond with attributes referenced by attr. If attr is NULL, the
// default condition variable attributes shall be used; the effect is the same
// as passing the address of a default condition variable attributes object.
// Upon successful initialization, the state of the condition variable shall
// become initialized.
//
// If successful, the pthread_cond_destroy() and pthread_cond_init() functions
// shall return zero; otherwise, an error number shall be returned to indicate
// the error.
//
// int pthread_cond_init(pthread_cond_t *restrict cond, const pthread_condattr_t *restrict attr);
func Xpthread_cond_init(tls *TLS, cond, attr uintptr) int32 {
	if attr != 0 {
		panic(todo(""))
	}

	pthreadCondsMu.Lock()

	defer pthreadCondsMu.Unlock()

	pthreadConds[cond] = &pthreadCond{}
	return 0
}

// int pthread_cond_wait(pthread_cond_t *restrict cond, pthread_mutex_t *restrict mutex);
func Xpthread_cond_wait(tls *TLS, cond, mutex uintptr) int32 {
	panic(todo(""))
}

// int pthread_cond_destroy(pthread_cond_t *cond);
func Xpthread_cond_destroy(tls *TLS, cond uintptr) int32 {
	pthreadCondsMu.Lock()

	defer pthreadCondsMu.Unlock()

	if pthreadConds[cond] != nil {
		delete(pthreadConds, cond)
		return 0
	}

	return errno.EINVAL
}

// int pthread_mutex_destroy(pthread_mutex_t *mutex);
func Xpthread_mutex_destroy(tls *TLS, mutex uintptr) int32 {
	pthreadMutexesMu.Lock()

	defer pthreadMutexesMu.Unlock()

	if pthreadMutexes[mutex] != nil {
		delete(pthreadMutexes, mutex)
		return 0
	}

	return errno.EINVAL
}

// int pthread_mutex_trylock(pthread_mutex_t *mutex);
func Xpthread_mutex_trylock(tls *TLS, mutex uintptr) int32 {
	panic(todo(""))
}

// int pthread_cond_broadcast(pthread_cond_t *cond);
func Xpthread_cond_broadcast(tls *TLS, cond uintptr) int32 {
	pthreadCondsMu.Lock()

	defer pthreadCondsMu.Unlock()

	if x := pthreadConds[cond]; x != nil {
		x.Broadcast()
		return 0
	}

	return errno.EINVAL
}

// pthread_t pthread_self(void);
func Xpthread_self(t *TLS) pthread.Pthread_t {
	panic(todo(""))
}

// int pthread_equal(pthread_t t1, pthread_t t2);
func Xpthread_equal(t *TLS, t1, t2 pthread.Pthread_t) int32 {
	panic(todo(""))
}
