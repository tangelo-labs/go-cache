// Package rsmcache provides a simple in-memory cache implementation that
// synchronizes using a Redis pub/sub channel.
//
// Use this to keep memory in-sync across multiple instances of your
// application.
//
// Each time a new value is written in the cache, it will publish a message to
// the configured Redis channel. All other instances of the application will
// receive this message and update their local cache.
package rsmcache
