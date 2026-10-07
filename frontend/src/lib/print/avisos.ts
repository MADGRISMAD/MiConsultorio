import type { Issuer, Patient } from '$lib/types';
import { doc, e, fmtDate, fullName, issuerBlock } from './base';

function responsable(c: Issuer) {
  const l = c.legal ?? ({} as Issuer['legal']);
  const name = l.privacy_contact || l.responsible_name || c.name;
  const address = l.privacy_address || c.address;
  const contact = [l.privacy_email && `correo ${l.privacy_email}`, (l.privacy_phone || c.phone) && `teléfono ${l.privacy_phone || c.phone}`].filter(Boolean).join(', ');
  // Public ARCO form of the clinic (absolute so it works on paper and in PDFs).
  const arcoUrl = c.arco_slug && typeof location !== 'undefined' ? `${location.origin}/arco/${c.arco_slug}` : '';
  return { name, address, contact, arcoUrl };
}

const subjectName = (p: Patient | null, animal: boolean) =>
  p ? (animal ? `${fullName(p)} (propietario: ${p.guardian_name || '________________'})` : fullName(p)) : '';

export function privacyNoticeHtml(patient: Patient | null, c: Issuer): string {
  const animal = patient?.subject === 'animal' || (!patient && c.kind === 'VETERINARY');
  const r = responsable(c);
  const who = animal ? 'el propietario o responsable del animal' : 'el paciente (o su representante legal)';
  const body = `
<div class="head">${issuerBlock(c)}<div class="doc"><h1>Aviso de privacidad integral</h1><div class="small">Pacientes y ${animal ? 'propietarios' : 'usuarios de servicios de salud'}</div></div></div>
<h2>1. Responsable del tratamiento</h2>
<p><strong>${e(c.name)}</strong>, con domicilio en ${e(r.address || '________________')}, es responsable del tratamiento de sus datos personales conforme a la Ley Federal de Protección de Datos Personales en Posesión de los Particulares (LFPDPPP).${r.name ? ` Persona o área de contacto: ${e(r.name)}.` : ''}${r.contact ? ` Contacto: ${e(r.contact)}.` : ''}</p>
<h2>2. Datos personales que recabamos</h2>
<p>Datos de identificación (nombre, fecha de nacimiento, sexo, CURP), datos de contacto (teléfono, correo, domicilio) y <strong>datos personales sensibles de salud</strong> (antecedentes, motivo de consulta, signos y mediciones, exploración, diagnósticos, tratamientos y recetas)${animal ? '. Tratándose de animales, se recaban los datos del propietario y los datos clínicos del animal' : ''}. Cuando el paciente es menor de edad o no puede decidir por sí mismo, los datos se obtienen de su madre, padre, tutor o representante legal.</p>
<h2>3. Finalidades</h2>
<p><strong>Finalidades primarias (necesarias):</strong> prestar atención ${animal ? 'veterinaria' : 'médica'}; integrar, conservar y resguardar el expediente clínico conforme a la NOM-004-SSA3-2012; agendar y dar seguimiento a citas; expedir recetas e indicaciones; y realizar cobro y facturación de los servicios.</p>
<p><strong>Finalidades secundarias (opcionales):</strong> envío de recordatorios de citas, seguimiento y comunicación sobre su atención y servicios del establecimiento.</p>
<p><span class="cb"></span>No deseo que mis datos se usen para las finalidades secundarias. <span class="small">(Su negativa no afecta la atención que recibe.)</span></p>
<h2>4. Transferencias</h2>
<p>Sus datos no se venden ni se usan con fines de mercadotecnia por terceros. Solo podrán comunicarse a autoridades sanitarias o judiciales cuando la ley lo exija, y a otros prestadores de servicios de salud ${animal ? 'veterinarios ' : ''}cuando usted lo autorice para su atención. Estas transferencias que la ley permite o exige no requieren su consentimiento adicional.</p>
<h2>5. Derechos ARCO, revocación y limitación</h2>
<p>Usted puede ejercer sus derechos de <strong>Acceso, Rectificación, Cancelación y Oposición (ARCO)</strong>, revocar su consentimiento o limitar el uso o divulgación de sus datos, mediante solicitud por escrito dirigida a ${e(r.name)}${r.address ? `, en ${e(r.address)}` : ''}${r.contact ? ` o por ${e(r.contact)}` : ''}${r.arcoUrl ? `, o bien en línea en el formulario ${e(r.arcoUrl)}` : ''}. Indique su nombre, un medio para responderle, una descripción clara de lo que solicita y copia de una identificación. Responderemos en un plazo máximo de 20 días hábiles. La cancelación estará sujeta a los plazos de conservación obligatoria del expediente clínico (mínimo 5 años desde el último acto médico), durante los cuales los datos se bloquean.</p>
<h2>6. Cambios al aviso</h2>
<p>Cualquier modificación a este aviso se pondrá a su disposición en el establecimiento o por los medios de contacto que nos haya proporcionado.</p>
<h2>7. Consentimiento</h2>
<p class="nobreak"><span class="cb"></span><strong>Otorgo mi consentimiento expreso para el tratamiento de mis datos personales y de salud para las finalidades descritas.</strong></p>
<div class="nobreak" style="margin-top:14px">${patient ? `<p>${animal ? 'Propietario' : 'Paciente'}: <strong>${e(animal ? patient.guardian_name || '' : fullName(patient))}</strong>${animal ? `<br>Animal: ${e(subjectName(patient, false))}` : ''}</p>` : `<p>Nombre: ______________________________________________</p>`}
<p class="small">Firmado por ${who}.</p>
<div class="sigs"><div class="sig">Nombre y firma</div><div class="sig">Fecha: ____ / ____ / ________</div></div></div>`;
  return doc('Aviso de privacidad', body);
}

export function consentHtml(patient: Patient | null, c: Issuer, professional?: { name: string; cedula: string }): string {
  const animal = patient?.subject === 'animal' || (!patient && c.kind === 'VETERINARY');
  const psych = c.kind === 'PSYCHOLOGY';
  const blank = (n = 2) => Array.from({ length: n }, () => '<div class="line"></div>').join('');
  const block = (t: string) => `<h2>${t}</h2>${blank(2)}`;
  const personLine = patient
    ? animal
      ? `<p>Animal: <strong>${e(fullName(patient))}</strong> (${e(String(patient.profile?.species ?? patient.profile?.especie ?? ''))}${patient.profile?.breed || patient.profile?.raza ? `, ${e(String(patient.profile?.breed ?? patient.profile?.raza))}` : ''}). Propietario: <strong>${e(patient.guardian_name)}</strong>.</p>`
      : `<p>Paciente: <strong>${e(fullName(patient))}</strong>${patient.birth_date ? `, fecha de nacimiento ${e(fmtDate(patient.birth_date))}` : ''}.${patient.guardian_name ? ` Tutor o representante: ${e(patient.guardian_name)}.` : ''}</p>`
    : animal
      ? '<p>Animal (nombre, especie, raza): ______________________________________<br>Propietario: ______________________________________</p>'
      : '<p>Paciente: ______________________________________ Fecha de nacimiento: ____________</p>';
  const decl = animal
    ? ['Se me explicó, en lenguaje claro, el procedimiento o tratamiento propuesto para mi animal, sus riesgos, beneficios y alternativas.', 'Tuve la oportunidad de hacer preguntas y fueron resueltas.', 'Como propietario o responsable, autorizo al Médico Veterinario Zootecnista y a su equipo a realizarlo.', 'Entiendo que puedo revocar esta autorización antes de que el procedimiento inicie.']
    : psych
      ? ['Se me explicó, en lenguaje claro, en qué consiste el proceso psicológico, sus beneficios, posibles molestias y alternativas.', 'Entiendo que lo que hable en sesión es confidencial, con excepciones: riesgo para mi vida o la de otras personas, y cuando una autoridad lo ordene por mandato legal.', 'Tuve la oportunidad de hacer preguntas y fueron resueltas.', 'Entiendo que puedo retirar mi consentimiento y dejar el proceso en cualquier momento.']
      : ['Se me explicó, en lenguaje comprensible, el procedimiento o tratamiento propuesto, sus riesgos, beneficios y alternativas.', 'Tuve la oportunidad de hacer preguntas y fueron resueltas.', 'Autorizo al profesional de la salud y a su equipo a realizarlo, y a atender contingencias o urgencias derivadas del mismo.', 'Entiendo que puedo revocar este consentimiento antes de que el procedimiento se realice.'];
  const body = `
<div class="head">${issuerBlock(c)}<div class="doc"><h1>Carta de consentimiento informado</h1><div class="small">Bajo información (NOM-004-SSA3-2012)</div></div></div>
${personLine}
<p>Lugar y fecha: ______________________________, a ____ de ______________ de ________</p>
${block('Procedimiento o tratamiento propuesto')}
${block('Riesgos y posibles complicaciones')}
${block('Beneficios esperados')}
${block('Alternativas')}
<h2>Declaraciones</h2>
<ol style="padding-left:20px">${decl.map((d) => `<li>${e(d)}</li>`).join('')}</ol>
<div class="sigs nobreak">
<div class="sig">${animal ? 'Propietario o responsable del animal' : 'Paciente, tutor o representante legal'}<br>Nombre y firma</div>
<div class="sig">${e(professional?.name || 'Profesional de la salud')}${professional?.cedula ? `<br>Céd. prof. ${e(professional.cedula)}` : '<br>Nombre, cédula profesional y firma'}</div>
<div class="sig">Testigo 1<br>Nombre y firma</div><div class="sig">Testigo 2<br>Nombre y firma</div></div>`;
  return doc('Consentimiento informado', body);
}
