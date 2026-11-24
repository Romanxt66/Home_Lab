package httpx

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// LimitarPorIP permite como mucho `max` peticiones por IP en cada ventana de
// tiempo y responde 429 a partir de ahí. Es una ventana fija en memoria: basta
// para frenar abusos en los endpoints públicos (registro y token de OAuth) sin
// montar infraestructura; con varias réplicas cada una contaría por su lado.
func LimitarPorIP(max int, ventana time.Duration) gin.HandlerFunc {
	type cuenta struct {
		n     int
		desde time.Time
	}
	var (
		mu             sync.Mutex
		visto          = map[string]*cuenta{}
		ultimaLimpieza = time.Now()
	)
	return func(c *gin.Context) {
		ahora := time.Now()
		ip := c.ClientIP()

		mu.Lock()
		// De vez en cuando se tiran las cuentas viejas para que el mapa no crezca sin fin.
		if ahora.Sub(ultimaLimpieza) > ventana {
			for k, v := range visto {
				if ahora.Sub(v.desde) > ventana {
					delete(visto, k)
				}
			}
			ultimaLimpieza = ahora
		}
		v := visto[ip]
		if v == nil || ahora.Sub(v.desde) > ventana {
			v = &cuenta{desde: ahora}
			visto[ip] = v
		}
		v.n++
		excedido := v.n > max
		restante := ventana - ahora.Sub(v.desde)
		mu.Unlock()

		if excedido {
			c.Header("Retry-After", itoa(int(restante.Seconds())+1))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "temporarily_unavailable", "error_description": "Demasiadas peticiones: espera un momento.",
			})
			return
		}
		c.Next()
	}
}

func itoa(n int) string {
	if n <= 0 {
		return "1"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
