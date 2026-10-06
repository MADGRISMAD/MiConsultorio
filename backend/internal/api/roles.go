package api

// Roles. A person's role is the single source of truth for what they can do:
// there are no per-user permission lists to get out of sync.
const (
	// Clinic roles
	RoleAdmin     = "admin"     // owner / administrator of the clinic
	RoleDoctor    = "doctor"    // physician, veterinarian or specialist
	RoleReception = "reception" // front desk and patient care staff
	RoleCashier   = "cashier"   // billing

	// Platform roles: staff of the product itself, outside any clinic
	RolePlatformAdmin   = "platform_admin"   // full control of clinics, subscriptions and staff
	RolePlatformSupport = "platform_support" // read-only help desk
)

var (
	clinicRoles   = []string{RoleAdmin, RoleDoctor, RoleReception, RoleCashier}
	platformRoles = []string{RolePlatformAdmin, RolePlatformSupport}
)

// roleLabels are the names shown to people.
var roleLabels = map[string]string{
	RoleAdmin:           "Administrador",
	RoleDoctor:          "Médico / especialista",
	RoleReception:       "Recepción",
	RoleCashier:         "Cajero",
	RolePlatformAdmin:   "Administrador de plataforma",
	RolePlatformSupport: "Soporte",
}

func isPlatformRole(role string) bool { return hasPermission(platformRoles, role) }
func isClinicRole(role string) bool   { return hasPermission(clinicRoles, role) }

// Capabilities (the names the frontend checks) granted by each clinic role.
//
//	admin      everything, including the team
//	doctor     sees the agenda, reads and writes clinical records
//	reception  sees and manages the agenda, no clinical records
//	cashier    sees the agenda (billing arrives with the POS)
var rolePermissions = map[string][]string{
	RoleAdmin:     {PermAdminUsers, PermAdminAppointments, PermAdminHistorials, PermNavHistorials, PermNavAppointments},
	RoleDoctor:    {PermNavAppointments, PermNavHistorials, PermAdminHistorials},
	RoleReception: {PermNavAppointments, PermAdminAppointments},
	RoleCashier:   {PermNavAppointments},
}

func permissionsFor(role string) []string {
	return append([]string{}, rolePermissions[role]...) // never nil, never aliased
}
