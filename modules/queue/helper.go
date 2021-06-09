// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"reflect"

	"github.com/goccy/go-json"
)

// Mappable represents an interface that can MapTo another interface
type Mappable interface {
	MapTo(v interface{}) error
}

// toConfig will attempt to convert a given configuration cfg into the provided exemplar type.
//
// It will tolerate the cfg being passed as a []byte or string of a json representation of the
// exemplar or the correct type of the exemplar itself
func toConfig(exemplar, cfg interface{}) (interface{}, error) {

	// First of all check if we've got the same type as the exemplar - if so it's all fine.
	if reflect.TypeOf(cfg).AssignableTo(reflect.TypeOf(exemplar)) {
		return cfg, nil
	}

	// Now if not - does it provide a MapTo function we can try?
	if mappable, ok := cfg.(Mappable); ok {
		newVal := reflect.New(reflect.TypeOf(exemplar))
		if err := mappable.MapTo(newVal.Interface()); err == nil {
			return newVal.Elem().Interface(), nil
		}
		// MapTo has failed us ... let's try the json route ...
	}

	// OK we've been passed a byte array right?
	configBytes, ok := cfg.([]byte)
	if !ok {
		// oh ... it's a string then?
		var configStr string

		configStr, ok = cfg.(string)
		configBytes = []byte(configStr)
	}
	if !ok {
		// hmm ... can we marshal it to json?
		var err error

		configBytes, err = json.Marshal(cfg)
		ok = err == nil
	}
	if !ok {
		// no ... we've tried hard enough at this point - throw an error!
		return nil, ErrInvalidConfiguration{cfg: cfg}
	}

	// OK unmarshal the byte array into a new copy of the exemplar
	newVal := reflect.New(reflect.TypeOf(exemplar))
	if err := json.Unmarshal(configBytes, newVal.Interface()); err != nil {
		// If we can't unmarshal it then return an error!
		return nil, ErrInvalidConfiguration{cfg: cfg, err: err}
	}
	return newVal.Elem().Interface(), nil
}

// unmarshalAs will attempt to unmarshal provided bytes as the provided exemplar
func unmarshalAs(bs []byte, exemplar interface{}) (data Data, err error) {
	if exemplar != nil {
		t := reflect.TypeOf(exemplar)
		n := reflect.New(t)
		ne := n.Elem()
		err = json.Unmarshal(bs, ne.Addr().Interface())
		data = ne.Interface().(Data)
	} else {
		err = json.Unmarshal(bs, &data)
	}

	return
}

// assignableTo will check if provided data is assignable to the same type as the exemplar
// if the provided exemplar is nil then it will always return true
func assignableTo(data Data, exemplar interface{}) bool {
	if exemplar == nil {
		return true
	}

	// Assert data is of same type as exemplar
	t := reflect.TypeOf(data)
	exemplarType := reflect.TypeOf(exemplar)

	return t.AssignableTo(exemplarType) && data != nil
}
