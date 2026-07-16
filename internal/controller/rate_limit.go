package controller

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"cliente-api/internal/view"
)

const (
	authRequestsPerMinute    = 120.0
	authRequestCapacity      = 20.0
	sessionRequestsPerMinute = 240.0
	sessionRequestCapacity   = 20.0
)

// rateLimitBucket guarda os créditos e tempos de um único cliente do limitador.
type rateLimitBucket struct {
	tokens   float64
	updated  time.Time
	lastSeen time.Time
}

// ipRateLimiter mantém os baldes em memória e serializa alterações concorrentes.
type ipRateLimiter struct {
	mutex           sync.Mutex
	buckets         map[string]*rateLimitBucket
	lastCleanup     time.Time
	now             func() time.Time
	refillPerSecond float64
	capacity        float64
}

// newIPRateLimiter cria um limitador configurável com créditos independentes
// para cada endereço IP. A capacidade define quantas requisições podem acontecer
// de uma vez; requestsPerMinute define a velocidade de reposição dos créditos.
func newIPRateLimiter(requestsPerMinute, capacity float64) *ipRateLimiter {
	if requestsPerMinute <= 0 || capacity < 1 {
		panic("a taxa por minuto e a capacidade do limitador devem ser positivas")
	}
	return &ipRateLimiter{
		buckets:         make(map[string]*rateLimitBucket),
		now:             time.Now,
		refillPerSecond: requestsPerMinute / 60,
		capacity:        capacity,
	}
}

// middleware aplica o balde do IP direto à requisição e devolve 429 com
// Retry-After quando esgotado. A topologia de proxy deve ser configurada na
// infraestrutura, sem confiar em cabeçalhos enviados pelo cliente.
func (limiter *ipRateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		allowed, retryAfter := limiter.allow(clientIP(request))
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			_ = view.WriteError(
				w,
				http.StatusTooManyRequests,
				"rate_limited",
				"muitas tentativas; aguarde antes de tentar novamente",
			)
			return
		}
		next.ServeHTTP(w, request)
	})
}

// allow consome um token do balde indicado, repõe créditos com o tempo e calcula Retry-After.
// O mutex torna a estrutura segura quando várias requisições do mesmo processo chegam em paralelo.
func (limiter *ipRateLimiter) allow(key string) (bool, int) {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	now := limiter.now()
	bucket, exists := limiter.buckets[key]
	if !exists {
		bucket = &rateLimitBucket{tokens: limiter.capacity, updated: now}
		limiter.buckets[key] = bucket
	}

	elapsed := now.Sub(bucket.updated).Seconds()
	if elapsed > 0 {
		// Cada segundo devolve uma fração dos créditos, sem ultrapassar a capacidade.
		bucket.tokens = math.Min(limiter.capacity, bucket.tokens+elapsed*limiter.refillPerSecond)
	}
	bucket.updated = now
	bucket.lastSeen = now

	if now.Sub(limiter.lastCleanup) >= 10*time.Minute {
		// IPs inativos são descartados para que o mapa não cresça indefinidamente.
		for bucketKey, candidate := range limiter.buckets {
			if now.Sub(candidate.lastSeen) >= 30*time.Minute {
				delete(limiter.buckets, bucketKey)
			}
		}
		limiter.lastCleanup = now
	}

	if bucket.tokens >= 1 {
		bucket.tokens--
		return true, 0
	}
	retryAfter := int(math.Ceil((1 - bucket.tokens) / limiter.refillPerSecond))
	if retryAfter < 1 {
		retryAfter = 1
	}
	return false, retryAfter
}

// clientIP extrai o host de RemoteAddr sem confiar em cabeçalhos encaminhados pelo cliente.
func clientIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if request.RemoteAddr == "" {
		return "unknown"
	}
	return request.RemoteAddr
}
