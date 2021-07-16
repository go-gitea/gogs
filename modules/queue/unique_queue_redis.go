// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"

	"github.com/go-redis/redis/v8"
)

// RedisUniqueQueueType is the type for redis queue
const RedisUniqueQueueType Type = "unique-redis"

// RedisUniqueQueue redis queue
type RedisUniqueQueue struct {
	*ByteFIFOUniqueQueue
}

// RedisUniqueQueueConfiguration is the configuration for the redis queue
type RedisUniqueQueueConfiguration struct {
	ByteFIFOQueueConfiguration
	RedisUniqueByteFIFOConfiguration
}

// NewRedisUniqueQueue creates single redis or cluster redis queue.
//
// Please note that this Queue does not guarantee that a particular
// task cannot be processed twice or more at the same time. Uniqueness is
// only guaranteed whilst the task is waiting in the queue.
func NewRedisUniqueQueue(handle HandlerFunc, cfg, exemplar interface{}) (Queue, error) {
	configInterface, err := toConfig(RedisUniqueQueueConfiguration{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(RedisUniqueQueueConfiguration)

	byteFIFO, err := NewRedisUniqueByteFIFO(config.RedisUniqueByteFIFOConfiguration)
	if err != nil {
		return nil, err
	}

	if len(byteFIFO.setName) == 0 {
		byteFIFO.setName = byteFIFO.queueName + "_unique"
	}

	byteFIFOQueue, err := NewByteFIFOUniqueQueue(RedisUniqueQueueType, byteFIFO, handle, config.ByteFIFOQueueConfiguration, exemplar)
	if err != nil {
		return nil, err
	}

	queue := &RedisUniqueQueue{
		ByteFIFOUniqueQueue: byteFIFOQueue,
	}

	queue.qid = GetManager().Add(queue, RedisUniqueQueueType, config, exemplar)

	return queue, nil
}

var _ UniqueByteFIFO = &RedisUniqueByteFIFO{}

// RedisUniqueByteFIFO represents a UniqueByteFIFO formed from a redisClient
type RedisUniqueByteFIFO struct {
	RedisByteFIFO
	setName string
}

// RedisUniqueByteFIFOConfiguration is the configuration for the RedisUniqueByteFIFO
type RedisUniqueByteFIFOConfiguration struct {
	RedisByteFIFOConfiguration
	SetName string
}

// NewRedisUniqueByteFIFO creates a UniqueByteFIFO formed from a redisClient
func NewRedisUniqueByteFIFO(config RedisUniqueByteFIFOConfiguration) (*RedisUniqueByteFIFO, error) {
	internal, err := NewRedisByteFIFO(config.RedisByteFIFOConfiguration)
	if err != nil {
		return nil, err
	}

	fifo := &RedisUniqueByteFIFO{
		RedisByteFIFO: *internal,
		setName:       config.SetName,
	}

	return fifo, nil
}

// PushFunc pushes data to the end of the fifo and calls the callback if it is added
func (fifo *RedisUniqueByteFIFO) PushFunc(ctx context.Context, data []byte, fn func() error) error {
	added, err := fifo.client.SAdd(ctx, fifo.setName, data).Result()
	if err != nil {
		return err
	}
	if added == 0 {
		return ErrAlreadyInQueue
	}
	if fn != nil {
		if err := fn(); err != nil {
			return err
		}
	}
	return fifo.client.RPush(ctx, fifo.queueName, data).Err()
}

// PushBack pushes data to the top of the fifo
func (fifo *RedisUniqueByteFIFO) PushBack(ctx context.Context, data []byte) error {
	added, err := fifo.client.SAdd(ctx, fifo.setName, data).Result()
	if err != nil {
		return err
	}
	if added == 0 {
		return ErrAlreadyInQueue
	}
	return fifo.client.LPush(ctx, fifo.queueName, data).Err()
}

// Pop pops data from the start of the fifo
func (fifo *RedisUniqueByteFIFO) Pop(ctx context.Context) ([]byte, error) {
	data, err := fifo.client.LPop(ctx, fifo.queueName).Bytes()
	if err != nil && err != redis.Nil {
		return data, err
	}

	if len(data) == 0 {
		return data, nil
	}

	err = fifo.client.SRem(ctx, fifo.setName, data).Err()
	return data, err
}

// Has returns whether the fifo contains this data
func (fifo *RedisUniqueByteFIFO) Has(ctx context.Context, data []byte) (bool, error) {
	return fifo.client.SIsMember(ctx, fifo.setName, data).Result()
}

func init() {
	queuesMap[RedisUniqueQueueType] = NewRedisUniqueQueue
}
