/** Public-domain screening questionnaires (Spanish). They orient the professional; they do not diagnose. */
export interface ScaleDef {
  id: 'phq9' | 'gad7' | 'pss10';
  name: string;
  topic: string;
  intro: string;
  options: string[];
  items: string[];
  /** highest possible score, for the chart */
  max: number;
}

export const SCALES: ScaleDef[] = [
  {
    id: 'phq9',
    name: 'PHQ-9',
    topic: 'Depresión',
    intro: 'Durante las últimas 2 semanas, ¿qué tan seguido ha tenido molestias debido a los siguientes problemas?',
    options: ['Para nada', 'Varios días', 'Más de la mitad de los días', 'Casi todos los días'],
    items: [
      'Poco interés o placer en hacer cosas',
      'Se ha sentido decaído(a), deprimido(a) o sin esperanzas',
      'Dificultad para quedarse o permanecer dormido(a), o dormir demasiado',
      'Se ha sentido cansado(a) o con poca energía',
      'Sin apetito o ha comido en exceso',
      'Se ha sentido mal con usted mismo(a), o que es un fracaso o que ha quedado mal con usted mismo(a) o con su familia',
      'Dificultad para concentrarse en cosas, tales como leer el periódico o ver televisión',
      '¿Se ha estado moviendo o hablando tan lento que otras personas podrían notarlo? O lo contrario: muy inquieto(a) o agitado(a), moviéndose mucho más de lo normal',
      'Pensamientos de que estaría mejor muerto(a) o de lastimarse de alguna manera'
    ],
    max: 27
  },
  {
    id: 'gad7',
    name: 'GAD-7',
    topic: 'Ansiedad',
    intro: 'Durante las últimas 2 semanas, ¿con qué frecuencia le han molestado los siguientes problemas?',
    options: ['Para nada', 'Varios días', 'Más de la mitad de los días', 'Casi todos los días'],
    items: [
      'Sentirse nervioso(a), ansioso(a) o con los nervios de punta',
      'No poder dejar de preocuparse o no poder controlar la preocupación',
      'Preocuparse demasiado por diferentes cosas',
      'Dificultad para relajarse',
      'Estar tan inquieto(a) que es difícil permanecer sentado(a) tranquilamente',
      'Molestarse o ponerse irritable fácilmente',
      'Sentir miedo como si algo terrible pudiera pasar'
    ],
    max: 21
  },
  {
    id: 'pss10',
    name: 'PSS-10',
    topic: 'Estrés percibido',
    intro: 'Las preguntas se refieren a sus sentimientos y pensamientos durante el último mes. Indique con qué frecuencia se sintió o pensó de cierta manera.',
    options: ['Nunca', 'Casi nunca', 'De vez en cuando', 'A menudo', 'Muy a menudo'],
    items: [
      '¿Con qué frecuencia ha estado afectado por algo que ha ocurrido inesperadamente?',
      '¿Con qué frecuencia se ha sentido incapaz de controlar las cosas importantes en su vida?',
      '¿Con qué frecuencia se ha sentido nervioso o estresado?',
      '¿Con qué frecuencia ha estado seguro sobre su capacidad para manejar sus problemas personales?',
      '¿Con qué frecuencia ha sentido que las cosas le van bien?',
      '¿Con qué frecuencia ha sentido que no podía afrontar todas las cosas que tenía que hacer?',
      '¿Con qué frecuencia ha podido controlar las dificultades de su vida?',
      '¿Con qué frecuencia ha sentido que tenía todo bajo control?',
      '¿Con qué frecuencia ha estado enfadado porque las cosas que le han ocurrido estaban fuera de su control?',
      '¿Con qué frecuencia ha sentido que las dificultades se acumulaban tanto que no podía superarlas?'
    ],
    max: 40
  }
];

export const scaleById = (id: string) => SCALES.find((s) => s.id === id);
