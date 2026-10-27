package inventory

import (
	"strings"
	"testing"
	"time"
)

func texto(s string) *string { return &s }

func TestValidateLocation(t *testing.T) {
	base := LocationInput{Name: "Garaje", Kind: KindRoom}

	t.Run("correcta", func(t *testing.T) {
		if res := ValidateLocation(base); !res.OK {
			t.Fatalf("debería valer: %s", res.Error)
		}
	})

	t.Run("nombre vacío", func(t *testing.T) {
		in := base
		in.Name = "   "
		if res := ValidateLocation(in); res.OK {
			t.Error("un nombre en blanco no debería pasar")
		}
	})

	t.Run("tipo desconocido", func(t *testing.T) {
		in := base
		in.Kind = "sotano"
		res := ValidateLocation(in)
		if res.OK || !strings.Contains(res.Error, "desconocido") {
			t.Errorf("se esperaba rechazo por tipo, se obtuvo: %+v", res)
		}
	})

	t.Run("limpia las notas vacías", func(t *testing.T) {
		in := base
		in.Notes = texto("   ")
		res := ValidateLocation(in)
		if !res.OK || res.Value.Notes != nil {
			t.Error("una nota en blanco debe guardarse como nulo, no como cadena vacía")
		}
	})
}

func TestBuildTree(t *testing.T) {
	casa := Location{ID: "1", Name: "Casa", Kind: KindPlace}
	salon := Location{ID: "2", Name: "Salón", Kind: KindRoom, ParentID: texto("1")}
	estante := Location{ID: "3", Name: "Estante", Kind: KindShelf, ParentID: texto("2")}
	huerfana := Location{ID: "4", Name: "Trastero", Kind: KindRoom, ParentID: texto("999")}

	arbol := BuildTree([]Location{estante, salon, casa, huerfana},
		map[string]int{"3": 7})

	if len(arbol) != 2 {
		t.Fatalf("se esperaban 2 raíces (Casa y la huérfana), hay %d", len(arbol))
	}

	// Orden alfabético: Casa antes que Trastero.
	if arbol[0].Name != "Casa" {
		t.Errorf("la primera raíz debería ser Casa, es %s", arbol[0].Name)
	}
	if len(arbol[0].Children) != 1 || arbol[0].Children[0].Name != "Salón" {
		t.Fatal("Salón debería colgar de Casa")
	}
	nieto := arbol[0].Children[0].Children
	if len(nieto) != 1 || nieto[0].Name != "Estante" {
		t.Fatal("Estante debería colgar de Salón")
	}
	if nieto[0].ItemCount != 7 {
		t.Errorf("el conteo de artículos no llegó: %d", nieto[0].ItemCount)
	}
}

func TestBuildTreeConCicloPropio(t *testing.T) {
	// Una ubicación que se declara madre de sí misma no debe desaparecer ni
	// colgar el programa: se trata como raíz.
	suya := Location{ID: "1", Name: "Rara", Kind: KindBox, ParentID: texto("1")}

	arbol := BuildTree([]Location{suya}, nil)
	if len(arbol) != 1 || arbol[0].Name != "Rara" {
		t.Error("una ubicación con ciclo propio debe salir como raíz")
	}
}

func TestDescendantIDs(t *testing.T) {
	locations := []Location{
		{ID: "1", Name: "Casa"},
		{ID: "2", Name: "Salón", ParentID: texto("1")},
		{ID: "3", Name: "Estante", ParentID: texto("2")},
		{ID: "4", Name: "Garaje"},
	}

	des := DescendantIDs("1", locations)

	for _, id := range []string{"1", "2", "3"} {
		if !des[id] {
			t.Errorf("%s debería contarse como descendiente de 1", id)
		}
	}
	if des["4"] {
		t.Error("el garaje no cuelga de la casa")
	}
}

func TestValidateItem(t *testing.T) {
	t.Run("rechaza cantidad negativa", func(t *testing.T) {
		res := ValidateItem(ItemInput{Name: "Tornillos", Quantity: -1})
		if res.OK {
			t.Error("una cantidad negativa no debería pasar")
		}
	})

	t.Run("normaliza los opcionales", func(t *testing.T) {
		res := ValidateItem(ItemInput{
			Name: "  Tornillos  ", Quantity: 5,
			Category: texto(""), Unit: texto("  u  "),
		})
		if !res.OK {
			t.Fatalf("debería valer: %s", res.Error)
		}
		if res.Value.Name != "Tornillos" {
			t.Errorf("nombre = %q", res.Value.Name)
		}
		if res.Value.Category != nil {
			t.Error("una categoría vacía debe quedar en nulo")
		}
		if res.Value.Unit == nil || *res.Value.Unit != "u" {
			t.Error("la unidad debería quedar sin espacios")
		}
	})
}

func TestMovementDelta(t *testing.T) {
	casos := []struct {
		kind     MovementKind
		cantidad int
		quiere   int
	}{
		{MovBought, 5, 5},
		{MovReceived, 3, 3},
		{MovSold, 2, -2},
		{MovGaveAway, 4, -4},
		{MovMoved, 9, 0},    // cambiar de sitio no altera el total
		{MovAdjust, -3, -3}, // el ajuste trae su propio signo
		{MovAdjust, 7, 7},
		{MovSold, -2, -2}, // el signo de entrada no debe invertir el sentido
	}

	for _, caso := range casos {
		t.Run(string(caso.kind), func(t *testing.T) {
			if got := MovementDelta(caso.kind, caso.cantidad); got != caso.quiere {
				t.Errorf("MovementDelta(%s, %d) = %d, se esperaba %d",
					caso.kind, caso.cantidad, got, caso.quiere)
			}
		})
	}
}

func TestValidateMovement(t *testing.T) {
	ahora := time.Now()
	base := MovementInput{ItemID: "i1", Kind: MovBought, Quantity: 5, OccurredAt: ahora}

	t.Run("compra correcta", func(t *testing.T) {
		res := ValidateMovement(base, 0)
		if !res.OK || res.Value.Delta != 5 {
			t.Fatalf("se esperaba delta 5: %+v", res)
		}
	})

	t.Run("no deja el stock en negativo", func(t *testing.T) {
		in := base
		in.Kind = MovSold
		in.Quantity = 10
		res := ValidateMovement(in, 3)
		if res.OK {
			t.Fatal("vender 10 teniendo 3 no debería permitirse")
		}
		if !strings.Contains(res.Error, "quedan 3") {
			t.Errorf("el mensaje debería decir cuánto queda: %q", res.Error)
		}
	})

	t.Run("el ajuste sí puede dejarlo en negativo… no", func(t *testing.T) {
		// El ajuste permite corregir, pero tampoco por debajo de cero.
		in := base
		in.Kind = MovAdjust
		in.Quantity = -10
		if res := ValidateMovement(in, 3); res.OK {
			t.Error("ni siquiera un ajuste debería dejar stock negativo")
		}
	})

	t.Run("mover exige destino", func(t *testing.T) {
		in := base
		in.Kind = MovMoved
		res := ValidateMovement(in, 5)
		if res.OK || !strings.Contains(res.Error, "destino") {
			t.Errorf("se esperaba exigir el destino: %+v", res)
		}
	})

	t.Run("mover con destino no cambia la cantidad", func(t *testing.T) {
		in := base
		in.Kind = MovMoved
		in.ToLocationID = texto("u2")
		res := ValidateMovement(in, 5)
		if !res.OK {
			t.Fatalf("debería valer: %s", res.Error)
		}
		if res.Value.Delta != 0 {
			t.Errorf("mover no altera el total, delta = %d", res.Value.Delta)
		}
	})

	t.Run("el destino se descarta si no es un movimiento", func(t *testing.T) {
		in := base
		in.ToLocationID = texto("u2")
		res := ValidateMovement(in, 0)
		if !res.OK || res.Value.ToLocationID != nil {
			t.Error("una compra no debería guardar ubicación de destino")
		}
	})

	t.Run("tipo desconocido", func(t *testing.T) {
		in := base
		in.Kind = "robado"
		if res := ValidateMovement(in, 10); res.OK {
			t.Error("un tipo inventado no debería pasar")
		}
	})

	t.Run("cantidad cero", func(t *testing.T) {
		in := base
		in.Quantity = 0
		if res := ValidateMovement(in, 10); res.OK {
			t.Error("comprar 0 unidades no tiene sentido")
		}
	})
}
