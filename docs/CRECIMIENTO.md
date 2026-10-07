# Gráficas de crecimiento: tablas de referencia

La pestaña **Crecimiento** del expediente dibuja la evolución de peso, talla, IMC y perímetro cefálico del paciente a partir de las consultas de la bitácora (`weight_kg`, `height_cm` y `head_circumference`; el IMC se calcula cuando una misma consulta trae peso y talla). La edad se calcula con la fecha de nacimiento.

**El producto no trae percentiles incluidos.** Las curvas y los valores percentil / Z solo aparecen cuando un administrador carga las tablas oficiales en **Ajustes > Tablas de crecimiento**. Sin ellas, la pestaña muestra únicamente las mediciones del paciente con el aviso «Carga las tablas oficiales de la OMS/CDC en Ajustes para ver percentiles».

Las tablas son de personas. Para animales solo se muestra la evolución del peso (no hay curvas por especie o raza).

## Dónde obtener las tablas oficiales

Descárgalas siempre de la fuente oficial y anota la dirección: se pide como «fuente» al cargarlas y queda en la bitácora de auditoría. Las direcciones pueden cambiar; si alguna ya no existe, busca el nombre del recurso en el sitio de la institución.

- **OMS, patrones de crecimiento infantil (0 a 5 años):** sitio de la OMS, sección *Child growth standards* (`https://www.who.int/tools/child-growth-standards`). Las tablas «expanded tables» traen los parámetros L, M y S por mes (o por día) de peso, longitud/talla, IMC y perímetro cefálico para la edad, por sexo.
- **OMS, referencia de crecimiento 5 a 19 años:** sitio de la OMS, sección *Growth reference data for 5-19 years* (`https://www.who.int/tools/growth-reference-data-for-5to19-years`). Trae peso, talla e IMC para la edad (no perímetro cefálico).
- **CDC (2 a 20 años, y 0 a 36 meses):** sitio del CDC, *Growth Charts, Data Table of ... / CDC Growth Charts Data Files* (`https://www.cdc.gov/growthcharts/cdc-data-files.htm`). Los archivos traen `Sex` (1 hombre, 2 mujer), `Agemos`, `L`, `M`, `S` y percentiles.

La OMS y el CDC son estándares distintos. Carga cada uno con su propio nombre (por ejemplo `OMS` y `CDC`); en la pestaña se elige cuál usar.

## Formato del CSV

Archivo de texto UTF-8, separado por comas, con encabezado en la primera fila. El punto es el separador decimal (no se acepta coma decimal ni separador de miles). Máximo 20,000 filas y 6 MB.

| Columna | Obligatoria | Descripción |
| --- | --- | --- |
| `indicator` | sí | `weight_for_age`, `length_height_for_age`, `bmi_for_age` o `head_circumference_for_age` |
| `sex` | sí | `M` (hombres / niños) o `F` (mujeres / niñas) |
| `age_months` | sí | Edad en meses, de 0 a 240. Admite decimales (por ejemplo `0.5`) |
| `l`, `m`, `s` | las tres juntas, o ninguna | Parámetros del método LMS. `m` y `s` deben ser mayores que cero |
| `p1`, `p3`, `p5`, `p10`, `p15`, `p25`, `p50`, `p75`, `p85`, `p90`, `p95`, `p97`, `p99` | opcionales | Valor del percentil. Deben ir en orden creciente dentro de la fila |

Reglas:

- Debe haber `l`, `m`, `s` o al menos una columna de percentil. Con `l`, `m` y `s` el Z-score y el percentil se calculan exactamente; solo con percentiles se interpolan entre los dos percentiles vecinos (y no hay valor fuera del rango de la propia tabla).
- Una combinación `indicator` + `sex` + `age_months` no puede repetirse.
- No se aceptan columnas desconocidas ni columnas repetidas.
- La unidad de cada indicador es la del expediente: peso en kg, talla y perímetro cefálico en cm, IMC en kg/m².
- Si el archivo trae edades en días, semanas o años, conviértelas a meses antes de cargar.

Ejemplo del formato (los números son inventados, solo ilustran las columnas; **no son valores de la OMS ni del CDC**):

```csv
indicator,sex,age_months,l,m,s
weight_for_age,M,0,1,3,0.1
weight_for_age,M,1,1,4,0.1
weight_for_age,F,0,1,3,0.1
```

Ejemplo con percentiles (igualmente inventado):

```csv
indicator,sex,age_months,p3,p50,p97
length_height_for_age,F,0,45,50,55
length_height_for_age,F,1,48,54,59
```

### Adaptar los archivos oficiales

Los archivos de la OMS y del CDC no vienen con estos nombres de columna. Antes de cargarlos hay que dejar una sola tabla con las columnas de arriba, por ejemplo en una hoja de cálculo:

- Un archivo por indicador y sexo se puede unir en uno solo agregando las columnas `indicator` y `sex`.
- CDC: `Sex` 1 es `M` y 2 es `F`; `Agemos` es `age_months`; `L`, `M`, `S` pasan a minúsculas.
- OMS: la columna de edad puede venir en meses (`Month`) o en días (`Day`); usa la de meses.

Revisa la vista previa que muestra el sistema antes de confirmar: indica cuántas filas hay por indicador y sexo y el rango de edades cubierto.

## Cómo funciona la carga

1. En Ajustes, **Tablas de crecimiento**: escribe el estándar (`OMS`, `CDC`), la fuente (nombre y dirección de donde se descargó) y elige el archivo.
2. **Revisar archivo**: el servidor valida de forma estricta columnas, valores y duplicados. Si hay errores, los lista con el número de línea y no guarda nada.
3. **Confirmar y cargar**: se guarda como una **versión nueva** del estándar. Nunca se sobrescribe ni se borra una carga anterior; para cada indicador y sexo se usa la versión más reciente que lo contenga.
4. Cada carga queda en la bitácora de auditoría con el estándar, la versión, la fuente, el número de filas y la huella SHA-256 del archivo.

Solo los administradores del consultorio pueden cargar tablas. Las tablas son por consultorio.

## Cálculo

Con LMS, el Z-score de una medición `x` es `((x/M)^L - 1) / (L·S)` (o `ln(x/M) / S` cuando `L = 0`), con L, M y S interpolados linealmente entre las dos edades vecinas de la tabla. El percentil es el de la distribución normal estándar de ese Z. Las curvas P3, P15, P50, P85 y P97 se obtienen invirtiendo la misma fórmula. Si la edad del paciente queda fuera del rango de la tabla no se calcula nada: no se extrapola.

Este cálculo no aplica el ajuste de la OMS para valores extremos (más allá de ±3 desviaciones estándar en peso y talla), así que en esos casos el Z puede diferir ligeramente del que publica la OMS. El percentil y el Z son una ayuda para la valoración; la interpretación clínica corresponde al profesional.
