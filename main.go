package main

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/go-redis/redis/v8"
    "github.com/neelp03/throttlex/ratelimiter"
    "github.com/neelp03/throttlex/store"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus metrics
var (
    requestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "requests_total",
            Help: "Total number of requests handled by the rate limiter",
        },
        []string{"client", "status"}, // Labels: client ID and request status (allowed/blocked)
    )
    rateLimitedRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "rate_limited_requests_total",
            Help: "Total number of rate-limited requests",
        },
        []string{"client"}, // Label: client ID
    )
)

func init() {
    // Register Prometheus metrics
    prometheus.MustRegister(requestsTotal)
    prometheus.MustRegister(rateLimitedRequestsTotal)
}

func main() {
    // Set up Redis client
    client := redis.NewClient(&redis.Options{
        Addr: "localhost:6379", // Redis running on localhost
    })
    err := client.Ping(context.Background()).Err()
    if err != nil {
        fmt.Println("Failed to connect to Redis:", err)
        return
    }

    // Initialize Redis store and fixed window rate limiter
    redisStore := store.NewRedisStore(client)
    limiter, err := ratelimiter.NewFixedWindowLimiter(redisStore, 10, time.Minute)
    if err != nil {
        fmt.Println("Failed to create rate limiter:", err)
        return
    }

    // Start Prometheus metrics endpoint in a separate goroutine
    go func() {
        http.Handle("/metrics", promhttp.Handler())
        fmt.Println("Serving metrics at :2112/metrics")
        if err := http.ListenAndServe(":2112", nil); err != nil {
            fmt.Println("Error starting metrics server:", err)
        }
    }()

    // Simulate requests from multiple clients (e.g., 1000 clients)
    clients := 1000
    for i := 0; i < clients; i++ {
        clientID := fmt.Sprintf("client%d", i+1)
        // Increase the number of requests each client makes to hit rate limit
        for j := 0; j < 25; j++ {  // Increase the request count to 25, 50, 100, 200, 500 per client
            allowed, err := limiter.Allow(clientID)
            if err != nil {
                fmt.Printf("Client %s - Error: %v\n", clientID, err)
                continue
            }

            if allowed {
                fmt.Printf("Client %s - Request %d allowed\n", clientID, j+1)
                requestsTotal.WithLabelValues(clientID, "allowed").Inc() // Increment allowed requests counter
            } else {
                fmt.Printf("Client %s - Request %d blocked\n", clientID, j+1)
                requestsTotal.WithLabelValues(clientID, "blocked").Inc() // Increment blocked requests counter
                rateLimitedRequestsTotal.WithLabelValues(clientID).Inc() // Increment rate-limited requests counter
            }

            time.Sleep(1 * time.Second) // Simulate 1 request per second
        }
    }
}
