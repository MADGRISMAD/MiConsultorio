// Fake Gemini API: answers by the kind of request, so the AI screens can be tried without a key.
import http from 'node:http';

const wrap = (obj) => ({ candidates: [{ content: { parts: [{ text: JSON.stringify(obj) }] } }] });

export function startFakeGemini({ port }) {
  const server = http.createServer(async (req, res) => {
    let body = '';
    for await (const c of req) body += c;
    let prompt = '';
    try {
      prompt = JSON.parse(body).contents[0].parts[0].text ?? '';
    } catch {}
    let out;
    if (prompt.includes('hoja de ejercicios domiciliarios')) {
      out = { items: [{ name: 'Puente de glúteo', sets: '3', reps: '12', notes: 'Sin arquear la espalda' }, { name: 'Gato-camello', sets: '2', reps: '10', notes: '' }], frequency: '1 vez al día', notes: 'Suspende si aumenta el dolor.' };
    } else if (prompt.includes('Ordena las notas sueltas')) {
      out = { subjective: 'Refiere dolor lumbar al sentarse', exam: 'Contractura paravertebral', assessment: 'Lumbalgia mecánica', plan: 'Calor local y ejercicios', measures: [{ key: 'pain_eva', value: '6' }] };
    } else if (prompt.includes('textos de la página pública')) {
      out = { title: 'Cuidamos tu salud', summary: '', sections: [{ heading: 'Motivos para elegirnos', items: ['Atención cercana', 'Citas puntuales'] }], text: 'Somos un consultorio con atención cercana.', lines: [{ name: '¿Cómo agendo una cita?', detail: 'Desde nuestra página, en línea.', qty: 1 }] };
    } else if (prompt.includes('Resume cómo va el consultorio')) {
      out = { title: 'Resumen', summary: 'Semana tranquila con pocas citas.', sections: [{ heading: 'Lo que va bien', items: ['Buena asistencia'] }], text: '', lines: [] };
    } else if (prompt.includes('TAREA:')) {
      out = { title: 'Mensaje de seguimiento', summary: 'Borrador listo.', sections: [{ heading: 'Puntos incluidos', items: ['Saludo', 'Invitación a escribir'] }], text: 'Hola [nombre], ¿cómo sigues? Escríbenos si tienes dudas. El equipo del consultorio.', lines: [] };
    } else {
      out = { overview: 'Resumen de prueba.', key_facts: [], pending: [], watch_for: [] };
    }
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(wrap(out)));
  });
  server.listen(port);
  return server;
}
