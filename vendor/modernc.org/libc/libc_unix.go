// Copyright 2020 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux darwin

package libc // import "modernc.org/libc"

import (
	"os"
	gosignal "os/signal"
	"reflect"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
	"modernc.org/libc/errno"
	"modernc.org/libc/poll"
	"modernc.org/libc/signal"
	"modernc.org/libc/stdio"
	"modernc.org/libc/sys/types"
)

// sighandler_t signal(int signum, sighandler_t handler);
func Xsignal(t *TLS, signum int32, handler uintptr) uintptr { //TODO use sigaction?
	signalsMu.Lock()

	defer signalsMu.Unlock()

	r := signals[signum]
	signals[signum] = handler
	switch handler {
	case signal.SIG_DFL:
		panic(todo("%v %#x", syscall.Signal(signum), handler))
	case signal.SIG_IGN:
		switch r {
		case signal.SIG_DFL:
			gosignal.Ignore(syscall.Signal(signum)) //TODO
		case signal.SIG_IGN:
			gosignal.Ignore(syscall.Signal(signum))
		default:
			panic(todo("%v %#x", syscall.Signal(signum), handler))
		}
	default:
		switch r {
		case signal.SIG_DFL:
			c := make(chan os.Signal, 1)
			gosignal.Notify(c, syscall.Signal(signum))
			go func() { //TODO mechanism to stop/cancel
				for {
					<-c
					var f func(*TLS, int32)
					*(*uintptr)(unsafe.Pointer(&f)) = handler
					tls := NewTLS()
					f(tls, signum)
					tls.Close()
				}
			}()
		case signal.SIG_IGN:
			panic(todo("%v %#x", syscall.Signal(signum), handler))
		default:
			panic(todo("%v %#x", syscall.Signal(signum), handler))
		}
	}
	return r
}

// void rewind(FILE *stream);
func Xrewind(t *TLS, stream uintptr) {
	Xfseek(t, stream, 0, stdio.SEEK_SET)
}

// int putchar(int c);
func Xputchar(t *TLS, c int32) int32 {
	if _, err := write([]byte{byte(c)}); err != nil {
		return stdio.EOF
	}

	return int32(c)
}

// int gethostname(char *name, size_t len);
func Xgethostname(t *TLS, name uintptr, slen types.Size_t) int32 {
	if slen < 0 {
		t.setErrno(errno.EINVAL)
		return -1
	}

	if slen == 0 {
		return 0
	}

	s, err := os.Hostname()
	if err != nil {
		panic(todo(""))
	}

	n := len(s)
	if len(s) >= int(slen) {
		n = int(slen) - 1
	}
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	copy((*RawMem)(unsafe.Pointer(name))[:n:n], (*RawMem)(unsafe.Pointer(sh.Data))[:n:n])
	*(*byte)(unsafe.Pointer(name + uintptr(n))) = 0
	return 0
}

// int remove(const char *pathname);
func Xremove(t *TLS, pathname uintptr) int32 {
	panic(todo(""))
}

// long pathconf(const char *path, int name);
func Xpathconf(t *TLS, path uintptr, name int32) long {
	panic(todo(""))
}

// ssize_t recvfrom(int sockfd, void *buf, size_t len, int flags, struct sockaddr *src_addr, socklen_t *addrlen);
func Xrecvfrom(t *TLS, sockfd int32, buf uintptr, len types.Size_t, flags int32, src_addr, addrlen uintptr) types.Ssize_t {
	panic(todo(""))
}

// ssize_t sendto(int sockfd, const void *buf, size_t len, int flags, const struct sockaddr *dest_addr, socklen_t addrlen);
func Xsendto(t *TLS, sockfd int32, buf uintptr, len types.Size_t, flags int32, src_addr uintptr, addrlen socklen_t) types.Ssize_t {
	panic(todo(""))
}

// void srand48(long int seedval);
func Xsrand48(t *TLS, seedval long) {
	panic(todo(""))
}

// long int lrand48(void);
func Xlrand48(t *TLS) long {
	panic(todo(""))
}

// ssize_t sendmsg(int sockfd, const struct msghdr *msg, int flags);
func Xsendmsg(t *TLS, sockfd int32, msg uintptr, flags int32) types.Ssize_t {
	panic(todo(""))
}

// int poll(struct pollfd *fds, nfds_t nfds, int timeout);
func Xpoll(t *TLS, fds uintptr, nfds poll.Nfds_t, timeout int32) int32 {
	if nfds == 0 {
		panic(todo(""))
	}

	// if dmesgs {
	// 	dmesg("%v: %#x %v %v, %+v", origin(1), fds, nfds, timeout, (*[1000]unix.PollFd)(unsafe.Pointer(fds))[:nfds:nfds])
	// }
	n, err := unix.Poll((*[1000]unix.PollFd)(unsafe.Pointer(fds))[:nfds:nfds], int(timeout))
	// if dmesgs {
	// 	dmesg("%v: %v %v", origin(1), n, err)
	// }
	if err != nil {
		t.setErrno(err)
		return -1
	}

	return int32(n)
}

// ssize_t recvmsg(int sockfd, struct msghdr *msg, int flags);
func Xrecvmsg(t *TLS, sockfd int32, msg uintptr, flags int32) types.Ssize_t {
	n, _, err := unix.Syscall(unix.SYS_RECVMSG, uintptr(sockfd), msg, uintptr(flags))
	if err != 0 {
		t.setErrno(err)
		return -1
	}

	return types.Ssize_t(n)
}

// struct cmsghdr *CMSG_NXTHDR(struct msghdr *msgh, struct cmsghdr *cmsg);
func X__cmsg_nxthdr(t *TLS, msgh, cmsg uintptr) uintptr {
	panic(todo(""))
}

// wchar_t *wcschr(const wchar_t *wcs, wchar_t wc);
func Xwcschr(t *TLS, wcs uintptr, wc wchar_t) wchar_t {
	panic(todo(""))
}

// gid_t getegid(void);
func Xgetegid(t *TLS) types.Gid_t {
	panic(todo(""))
}

// gid_t getgid(void);
func Xgetgid(t *TLS) types.Gid_t {
	panic(todo(""))
}

// void *shmat(int shmid, const void *shmaddr, int shmflg);
func Xshmat(t *TLS, shmid int32, shmaddr uintptr, shmflg int32) uintptr {
	panic(todo(""))
}

// int shmctl(int shmid, int cmd, struct shmid_ds *buf);
func Xshmctl(t *TLS, shmid, cmd int32, buf uintptr) int32 {
	panic(todo(""))
}

// int shmdt(const void *shmaddr);
func Xshmdt(t *TLS, shmaddr uintptr) int32 {
	panic(todo(""))
}

// int getpwnam_r(const char *name, struct passwd *pwd,
//                       char *buf, size_t buflen, struct passwd **result);
func Xgetpwnam_r(t *TLS, name, pwd, buf uintptr, buflen types.Size_t, result uintptr) int32 {
	panic(todo(""))
}

// int getpwuid_r(uid_t uid, struct passwd *pwd,
//                       char *buf, size_t buflen, struct passwd **result);
func Xgetpwuid_r(t *TLS, uid types.Uid_t, pwd, buf uintptr, buflen types.Size_t, result uintptr) int32 {
	panic(todo(""))
}

// int getresuid(uid_t *ruid, uid_t *euid, uid_t *suid);
func Xgetresuid(t *TLS, ruid, euid, suid uintptr) int32 {
	panic(todo(""))
}

// int getresgid(gid_t *rgid, gid_t *egid, gid_t *sgid);
func Xgetresgid(t *TLS, rgid, egid, sgid uintptr) int32 {
	panic(todo(""))
}
