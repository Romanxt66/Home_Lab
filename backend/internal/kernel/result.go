// Package kernel contiene las piezas que comparten todos los módulos y que no
// dependen de nada externo: ni de la base de datos, ni de HTTP, ni de Gin.
package kernel

// Result modela éxito o fallo sin lanzar pánico, igual que el Result<T,E> del
// Lab original. El dominio y la aplicación lo devuelven para que quien llama
// tenga que mirar el error; los adaptadores traducen los fallos del exterior.
//
// En Go lo idiomático sería devolver (T, error), y para los errores técnicos
// (red caída, base inaccesible) eso es justo lo que hacemos. Result se reserva
// para los fallos *de negocio* que el usuario debe leer: "el nombre no puede
// estar vacío". Son dos cosas distintas y conviene no mezclarlas.
type Result[T any] struct {
	OK    bool
	Value T
	Error string
}

// Ok envuelve un valor correcto.
func Ok[T any](value T) Result[T] {
	return Result[T]{OK: true, Value: value}
}

// Err envuelve un mensaje de error pensado para que lo lea una persona.
func Err[T any](message string) Result[T] {
	return Result[T]{OK: false, Error: message}
}
