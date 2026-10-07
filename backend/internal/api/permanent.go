package api

import (
	"slices"
	"strings"
)

// permanentAdmins son administradores de plataforma que nadie puede desactivar ni bajar de rol
// desde la app. Es la misma lista en MiColmena, MiTiendita y MiConsultorio; para cambiarla hay
// que cambiar el código, así queda en el historial de git.
var permanentAdmins = []string{"madgrismad@gmail.com", "mayra.bamaca09@gmail.com", "luispantoja1102@gmail.com"}

func isPermanentAdmin(email string) bool {
	return slices.Contains(permanentAdmins, strings.ToLower(strings.TrimSpace(email)))
}
