// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package services

import (
	"fmt"
	"sync"

	"code.gitea.io/gitea/modules/log"
)

// Service represents a service
type Service interface {
	Init() error
}

type serviceHandler struct {
	Name string
	Service
	DependsOn []string
}

var (
	services     = make(map[string]serviceHandler)
	servicesLock sync.RWMutex
)

// ServiceFunc is a wrap to make a function as a Service interface
type ServiceFunc func() error

// Init run the service init function
func (h ServiceFunc) Init() error {
	return h()
}

// RegisterService register a service, the name should be the package path, all services should be under modules/ or services/
// i.e. a package on modules/setting should be given the name setting, package modules/notification/webhook should be given notification/webhook
func RegisterService(name string, svr ServiceFunc, dependsOn ...string) {
	servicesLock.Lock()
	services[name] = serviceHandler{
		Name:      name,
		Service:   svr,
		DependsOn: dependsOn,
	}
	servicesLock.Unlock()
}

func initOne(initialized map[string]struct{}, services map[string]serviceHandler, service serviceHandler) error {
	if _, ok := initialized[service.Name]; ok {
		return nil
	}
	for _, depend := range service.DependsOn {
		if _, ok := services[depend]; !ok {
			return fmt.Errorf("Service %s dependent by %s is not exist", depend, service.Name)
		}
		if _, ok := initialized[depend]; !ok {
			if err := initOne(initialized, services, services[depend]); err != nil {
				return err
			}
		}
	}
	log.Trace("Init service %s", service.Name)
	if err := service.Init(); err != nil {
		return err
	}
	initialized[service.Name] = struct{}{}
	return nil
}

// Init initializes services according the sequence
func Init() error {
	servicesLock.Lock()
	defer servicesLock.Unlock()
	var initialized = make(map[string]struct{})
	for _, service := range services {
		if err := initOne(initialized, services, service); err != nil {
			return fmt.Errorf("Init service %s failed: %v", service.Name, err)
		}
	}
	return nil
}
