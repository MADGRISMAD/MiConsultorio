package api

import (
	"net/http"
	"strings"
)

// arcoLegalNote is shown wherever deadlines appear.
const arcoLegalNote = "Los plazos se calculan en días hábiles de lunes a viernes, sin considerar días festivos ni inhábiles del consultorio. " +
	"Son una guía operativa: verifica los plazos y el contenido de cada respuesta con tu asesor legal."

type arcoStep struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

// arcoPackage is the checklist, links and draft answer that help staff answer one request.
func (s *Server) arcoPackage(w http.ResponseWriter, r *http.Request) {
	a, ok := s.arcoFind(w, r, s.db, "")
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	l, err := loadLegal(r.Context(), s.db, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	clinic := s.arcoClinicName(r.Context(), p.ClinicID)
	linked := a.PatientID != nil
	archived := false
	if linked {
		_ = s.db.QueryRow(r.Context(), `SELECT archived_at IS NOT NULL FROM patients WHERE clinic_id = $1 AND id = $2`, p.ClinicID, *a.PatientID).Scan(&archived)
	}

	steps := []arcoStep{
		{"Verifica la identidad de quien solicita (identificación oficial; si actúa por otra persona, carta poder o representación legal).", a.IdentityVerified},
		{"Vincula el expediente del paciente para trabajar sobre sus datos.", linked},
	}
	out := map[string]any{
		"folio": a.Folio, "kind": a.Kind, "kind_label": arcoKindLabels[a.Kind],
		"legal_note": arcoLegalNote,
		"contact":    map[string]string{"name": l.PrivacyContact, "email": l.PrivacyEmail, "phone": l.PrivacyPhone, "address": l.PrivacyAddress},
	}
	if linked {
		out["patient"] = map[string]any{"id": *a.PatientID, "name": a.PatientName}
	}
	var draft string
	switch a.Kind {
	case "acceso":
		steps = append(steps,
			arcoStep{"Descarga la exportación del expediente (JSON con todo lo que el consultorio conserva sobre la persona) y revísala antes de entregarla.", false},
			arcoStep{"Entrega la copia por un medio seguro, directamente a la persona o a su representante; evita enviar datos de salud por correo sin protección.", false},
			arcoStep{"Registra la respuesta y marca la solicitud como atendida.", a.Status == "atendida"})
		if linked {
			out["export_path"] = "/patients/" + *a.PatientID + "/export"
		}
		draft = "Estimado(a) " + a.RequesterName + ":\n\nEn atención a su solicitud de acceso con folio " + a.Folio + ", " + clinic +
			" confirma que sí cuenta con datos personales suyos. Le haremos entrega de una copia de la información de su expediente por el medio que usted indicó, previa verificación de su identidad.\n\n" +
			"Para cualquier duda puede comunicarse con " + arcoContactLine(l) + "."
	case "rectificacion":
		steps = append(steps,
			arcoStep{"Pide el documento que acredita el dato correcto (acta, identificación, comprobante).", false},
			arcoStep{"Corrige el dato en el expediente. Las notas clínicas no se editan: si el error está en una nota, agrega una adenda que lo aclare (NOM-004).", false},
			arcoStep{"Informa a la persona qué dato se corrigió.", a.Status == "atendida"})
		draft = "Estimado(a) " + a.RequesterName + ":\n\nEn atención a su solicitud de rectificación con folio " + a.Folio + ", " + clinic +
			" corrigió los datos indicados en su expediente. Las notas clínicas ya firmadas se conservan sin cambios y, cuando es necesario, se agrega una adenda que aclara la información.\n\n" +
			"Para cualquier duda puede comunicarse con " + arcoContactLine(l) + "."
	case "cancelacion":
		out["retention_note"] = "NOM-004-SSA3-2012 obliga a conservar el expediente clínico al menos 5 años desde el último acto médico. " +
			"Por eso Caresia no borra expedientes: al cancelar se bloquea el uso de los datos (el expediente se archiva y deja de usarse) y se conservan solo durante el periodo legal. " +
			"Cumplido ese plazo, el consultorio decide con su asesor legal cómo suprimirlos."
		steps = append(steps,
			arcoStep{"Explica a la persona que el expediente se conserva el periodo que exige la NOM-004 y que, mientras tanto, se bloquea su uso.", false},
			arcoStep{"Bloquea el uso del expediente (botón «Archivar expediente»). No se borra ningún registro clínico.", archived},
			arcoStep{"Si hay datos que no forman parte del expediente (por ejemplo, contactos de recordatorios), suprímelos o desactívalos.", false})
		draft = "Estimado(a) " + a.RequesterName + ":\n\nEn atención a su solicitud de cancelación con folio " + a.Folio + ", " + clinic +
			" bloqueó el uso de sus datos personales. Le informamos que, por disposición de la NOM-004-SSA3-2012, el expediente clínico debe conservarse al menos 5 años desde el último acto médico; durante ese periodo los datos se mantienen bloqueados, sin darles ningún otro uso, y al concluirlo serán suprimidos.\n\n" +
			"Para cualquier duda puede comunicarse con " + arcoContactLine(l) + "."
	case "oposicion":
		steps = append(steps,
			arcoStep{"Identifica la finalidad a la que se opone (por ejemplo recordatorios o comunicaciones no necesarias para el servicio).", false},
			arcoStep{"Deja de usar los datos para esa finalidad (por ejemplo, desactiva recordatorios del paciente).", false},
			arcoStep{"La atención médica y la conservación del expediente continúan: la oposición no las elimina. Explícalo en la respuesta.", false})
		draft = "Estimado(a) " + a.RequesterName + ":\n\nEn atención a su solicitud de oposición con folio " + a.Folio + ", " + clinic +
			" dejó de tratar sus datos personales para la finalidad indicada. El tratamiento necesario para su atención médica y para conservar su expediente conforme a la NOM-004-SSA3-2012 continúa.\n\n" +
			"Para cualquier duda puede comunicarse con " + arcoContactLine(l) + "."
	case "revocacion":
		steps = append(steps,
			arcoStep{"Identifica el consentimiento que se revoca (por ejemplo, el de finalidades secundarias o el de recordatorios).", false},
			arcoStep{"Deja de usar los datos para esas finalidades y desactiva los recordatorios del paciente si aplica.", false},
			arcoStep{"Aclara que la revocación no tiene efectos retroactivos ni afecta el tratamiento necesario para la atención médica y la conservación del expediente.", false})
		draft = "Estimado(a) " + a.RequesterName + ":\n\nEn atención a su solicitud de revocación del consentimiento con folio " + a.Folio + ", " + clinic +
			" dejó de tratar sus datos personales para las finalidades que usted indicó. La revocación no afecta el tratamiento necesario para su atención médica ni la conservación de su expediente conforme a la NOM-004-SSA3-2012.\n\n" +
			"Para cualquier duda puede comunicarse con " + arcoContactLine(l) + "."
	}
	out["steps"] = steps
	out["draft_response"] = draft
	out["draft_denial"] = "Estimado(a) " + a.RequesterName + ":\n\nEn atención a su solicitud con folio " + a.Folio + ", " + clinic +
		" le informa que no es posible atenderla en los términos solicitados por el siguiente motivo: [MOTIVO]. " +
		"Puede comunicarse con " + arcoContactLine(l) + " para mayor información."

	_ = arcoEvent(r.Context(), s.db, p.ClinicID, a.ID, "package", p.actorName(), "Se generó el paquete de respuesta")
	writeJSON(w, http.StatusOK, map[string]any{"package": out})
}

func arcoContactLine(l Legal) string {
	parts := []string{}
	if l.PrivacyContact != "" {
		parts = append(parts, l.PrivacyContact)
	}
	if l.PrivacyEmail != "" {
		parts = append(parts, l.PrivacyEmail)
	}
	if l.PrivacyPhone != "" {
		parts = append(parts, l.PrivacyPhone)
	}
	if len(parts) == 0 {
		return "el consultorio"
	}
	return strings.Join(parts, ", ")
}
