package scraper

import (
	"context"
	"strings"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Pagina es lo que devuelve una descarga.
type Pagina struct {
	Status      int
	ContentType string
	Body        string
}

// Descargador trae la página. Se declara aquí, en quien lo usa.
type Descargador interface {
	Descargar(ctx context.Context, url string) kernel.Result[Pagina]
}

// Extractor saca del HTML lo que señale el selector.
type Extractor interface {
	Extraer(html, selector, atributo string) kernel.Result[[]string]
}

// Resultado es la salida del caso de uso: o los elementos encontrados, o un
// trozo del cuerpo si no se pidió selector.
type Resultado struct {
	Status      int
	ContentType string
	Elementos   []string
	Crudo       string
}

type Service struct {
	descargador Descargador
	extractor   Extractor
}

func NewService(d Descargador, e Extractor) *Service {
	return &Service{descargador: d, extractor: e}
}

func (s *Service) Ejecutar(ctx context.Context, direccion, selector, atributo string) kernel.Result[Resultado] {
	url := ValidarURL(direccion)
	if !url.OK {
		return kernel.Err[Resultado](url.Error)
	}

	pagina := s.descargador.Descargar(ctx, url.Value)
	if !pagina.OK {
		return kernel.Err[Resultado](pagina.Error)
	}

	selector = strings.TrimSpace(selector)
	if selector == "" {
		return kernel.Ok(Resultado{
			Status:      pagina.Value.Status,
			ContentType: pagina.Value.ContentType,
			Crudo:       Recortar(pagina.Value.Body, LimiteCrudo),
		})
	}

	elementos := s.extractor.Extraer(pagina.Value.Body, selector, strings.TrimSpace(atributo))
	if !elementos.OK {
		return kernel.Err[Resultado](elementos.Error)
	}

	return kernel.Ok(Resultado{
		Status:      pagina.Value.Status,
		ContentType: pagina.Value.ContentType,
		Elementos:   elementos.Value,
	})
}
