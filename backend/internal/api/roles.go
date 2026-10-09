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
//	cashier    sees the agenda; sells, runs the register and reads sales reports (plans with cobros)
var rolePermissions = map[string][]string{
	RoleAdmin:     {PermAdminUsers, PermAdminAppointments, PermAdminHistorials, PermNavHistorials, PermNavAppointments, PermPOS, PermPOSReports, PermPOSManage},
	RoleDoctor:    {PermNavAppointments, PermNavHistorials, PermAdminHistorials},
	RoleReception: {PermNavAppointments, PermAdminAppointments, PermPOS},
	RoleCashier:   {PermNavAppointments, PermPOS, PermPOSReports},
}

// grantablePerms are the capabilities an administrator can add to or take from one person. Managing the team and the
// administrator role itself are never grantable.
var grantablePerms = []string{PermNavAppointments, PermAdminAppointments, PermNavHistorials, PermAdminHistorials, PermPOS, PermPOSReports, PermPOSManage}

// permissionsWith is the role's capabilities plus what was added and minus what was taken from this person.
// Administrators ignore overrides. A capability that needs another one brings it (managing needs seeing), and
// taking the basic one takes the dependent ones too.
func permissionsWith(role string, extra, denied []string) []string {
	base := permissionsFor(role)
	if role == RoleAdmin || (len(extra) == 0 && len(denied) == 0) {
		return base
	}
	set := map[string]bool{}
	for _, p := range base {
		set[p] = true
	}
	for _, p := range extra {
		if hasPermission(grantablePerms, p) {
			set[p] = true
		}
	}
	// dependent -> basic
	for dep, basic := range map[string]string{PermAdminAppointments: PermNavAppointments, PermAdminHistorials: PermNavHistorials, PermPOSReports: PermPOS, PermPOSManage: PermPOS} {
		if set[dep] {
			set[basic] = true
		}
	}
	for _, p := range denied {
		delete(set, p)
	}
	for dep, basic := range map[string]string{PermAdminAppointments: PermNavAppointments, PermAdminHistorials: PermNavHistorials, PermPOSReports: PermPOS, PermPOSManage: PermPOS} {
		if !set[basic] {
			delete(set, dep)
		}
	}
	out := make([]string, 0, len(set))
	for _, p := range grantablePerms { // stable order
		if set[p] {
			out = append(out, p)
		}
	}
	return out
}

func permissionsFor(role string) []string {
	return append([]string{}, rolePermissions[role]...) // never nil, never aliased
}
