package scraper

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/cascadia"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

const (
	tiempoLimite = 15 * time.Second
	agente       = "Mozilla/5.0 (compatible; HomeLabScraper/1.0) AppleWebKit/537.36"

	// Tope de descarga. Sin él, una URL que sirva un archivo enorme se
	// comería la memoria del proceso entero.
	maxCuerpo = 8 << 20 // 8 MiB
)

// HTTPDescargador baja la página con la biblioteca estándar.
type HTTPDescargador struct {
	client *http.Client
}

func NewHTTPDescargador() *HTTPDescargador {
	return &HTTPDescargador{client: &http.Client{Timeout: tiempoLimite}}
}

func (d *HTTPDescargador) Descargar(ctx context.Context, direccion string) kernel.Result[Pagina] {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, direccion, nil)
	if err != nil {
		return kernel.Err[Pagina]("No se pudo preparar la petición.")
	}
	req.Header.Set("User-Agent", agente)
	req.Header.Set("Accept", "*/*")

	res, err := d.client.Do(req)
	if err != nil {
		// Distinguir el tiempo agotado del resto: la solución que tiene a
		// mano quien lo lee es distinta.
		if strings.Contains(err.Error(), "context deadline exceeded") ||
			strings.Contains(err.Error(), "Client.Timeout") {
			return kernel.Err[Pagina]("La página tardó más de 15 segundos en responder.")
		}
		return kernel.Err[Pagina]("No se pudo conectar con esa dirección.")
	}
	defer res.Body.Close()

	cuerpo, err := io.ReadAll(io.LimitReader(res.Body, maxCuerpo))
	if err != nil {
		return kernel.Err[Pagina]("Se cortó la descarga de la página.")
	}

	return kernel.Ok(Pagina{
		Status:      res.StatusCode,
		ContentType: res.Header.Get("Content-Type"),
		Body:        string(cuerpo),
	})
}

// GoqueryExtractor interpreta el HTML. Es el equivalente en Go de Cheerio,
// que es lo que usaba el Lab.
type GoqueryExtractor struct{}

func NewGoqueryExtractor() *GoqueryExtractor {
	return &GoqueryExtractor{}
}

func (GoqueryExtractor) Extraer(html, selector, atributo string) kernel.Result[[]string] {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return kernel.Err[[]string]("No se pudo interpretar el HTML.")
	}

	// Se compila el selector aparte en lugar de usar doc.Find, que ante un
	// selector mal escrito devuelve vacío sin decir nada: el usuario vería
	// "sin resultados" cuando lo que pasa es que su selector no vale.
	compilado, err := cascadia.Compile(selector)
	if err != nil {
		return kernel.Err[[]string]("Ese selector CSS no es válido.")
	}
	seleccion := doc.FindMatcher(compilado)

	elementos := make([]string, 0, min(seleccion.Length(), MaxElementos))
	seleccion.EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if len(elementos) >= MaxElementos {
			return false
		}

		var valor string
		if atributo != "" {
			valor, _ = s.Attr(atributo)
		} else {
			valor = Normalizar(s.Text())
		}

		if valor != "" {
			elementos = append(elementos, valor)
		}
		return true
	})

	return kernel.Ok(elementos)
}
