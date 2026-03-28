---
on:
  issues:
    types: [opened]

if: github.actor == github.repository_owner

# Modelos disponibles para el campo "model:", ordenados de menor a mayor coste.
# El coste se mide en "premium requests" del plan de GitHub Copilot.
#
# ── GRATUITOS (incluidos en planes de pago, multiplicador 0x) ──────────────────
#   gpt-4o          → GPT-4o. Rápido y equilibrado. Buena opción por defecto.
#   gpt-4.1         → GPT-4.1. Alta precisión en completaciones de código.
#   gpt-5-mini      → GPT-5 mini. Velocidad + calidad para la mayoría de tareas.
#
# ── PREMIUM - COSTE BAJO (multiplicador ~1x) ───────────────────────────────────
#   claude-haiku-4.5  → Claude Haiku 4.5. El más rápido y barato de Anthropic.
#   claude-sonnet-4.5 → Claude Sonnet 4.5. Equilibrio calidad/coste.
#   claude-sonnet-4-6 → Claude Sonnet 4.6. Razonamiento mejorado respecto a 4.5.
#
# ── PREMIUM - COSTE MEDIO (multiplicador ~1-2x) ────────────────────────────────
#   gemini-2.5-pro  → Gemini 2.5 Pro. Ideal para contextos largos y depuración.
#   gpt-5           → GPT-5. Razonamiento profundo y depuración avanzada.
#   gpt-5.1         → GPT-5.1. Mayor precisión en análisis multi-fichero.
#
# ── PREMIUM - COSTE ALTO (multiplicador ~3x) ───────────────────────────────────
#   claude-opus-4.5 → Claude Opus 4.5. Máxima capacidad de razonamiento (3x).
#   claude-opus-4-6 → Claude Opus 4.6. Versión mejorada de Opus 4.5 (3x+).
#
# Notas:
#   · Los multiplicadores y la disponibilidad de modelos pueden cambiar.
#   · Referencia oficial: https://docs.github.com/en/copilot/using-github-copilot/ai-models/supported-ai-models-in-copilot
model: gpt-4o

permissions:
  issues: write
  contents: read

safe-outputs:
  update-issue:
    title:
    body:

tools:
  github:
    toolsets: [issues]
---

# Mejorador de Issues

Mejora automáticamente los issues nuevos para que sean claros, estén bien estructurados y sean fáciles de entender.

## Issue a mejorar

<!-- Estas variables son sustituidas automáticamente por el motor de GitHub Copilot Agentic Workflows en tiempo de ejecución -->

| Campo  | Valor          |
| ------ | -------------- |
| Número | #$ISSUE_NUMBER |
| Autor  | @$ISSUE_AUTHOR |
| Título | $ISSUE_TITLE   |
| Cuerpo | $ISSUE_BODY    |

## Tus tareas

### 1. Obtener contexto

- Lee el README para entender el proyecto (es una CLI en Go para gestionar runbooks de desarrollo)
- Lista las etiquetas del repositorio (las necesitarás después)

### 2. Mejorar el título

Añade un emoji como prefijo según el tipo de issue:

- 🐛 Bug (algo no funciona)
- ✨ Enhancement (nueva mejora o funcionalidad)
- 📝 Documentation (documentación, README)
- ❓ Question (pregunta o duda)
- ⚙️ Build / CI (scripts, pipelines, automatización)
- 🔒 Security (vulnerabilidades o mejoras de seguridad)
- ♻️ Refactor (refactorización de código sin cambio funcional)
- 🧪 Tests (añadir o corregir tests)

Ejemplo: `🐛 Error al ejecutar devrune run en macOS`

### 3. Reestructurar el cuerpo

Usa secciones claras con encabezados emoji.

**Para bugs:**

```markdown
## 🐛 Descripción
(Qué está fallando)

## 📋 Pasos para reproducir
1. ...
2. ...
3. ...

## ✅ Comportamiento esperado
(Qué debería pasar)

## ❌ Comportamiento actual
(Qué pasa realmente)

## 🖥️ Entorno
- **OS**: (e.g., macOS 15, Ubuntu 24.04)
- **Go version**: (output de `go version`)
- **DevRune version**: (output de `devrune --version`)

## 📸 Capturas (si aplica)
(Imágenes, GIFs o logs del problema)
```

**Para mejoras/features:**

```markdown
## ✨ Descripción
(Qué se quiere añadir o mejorar)

## 🎯 ¿Por qué es necesario?
(Contexto y motivación)

## 📐 Solución propuesta
(Cómo se podría implementar)

## 📝 Notas adicionales
(Cualquier otra información relevante)
```

**Para documentación:**

```markdown
## 📝 Descripción
(Qué documentación falta o hay que mejorar)

## 📍 Ubicación
(Dónde debería estar la documentación)

## ✏️ Contenido sugerido
(Qué debería incluir)
```

### 4. Añadir pie de página

```markdown
---
> 🤖 *Issue mejorado automáticamente por Copilot. Autor original: @$ISSUE_AUTHOR*
```

### 5. Aplicar cambios

- **Actualiza** el issue #$ISSUE_NUMBER con el nuevo título y cuerpo
- **Asigna** 1-3 etiquetas relevantes de las disponibles en el repositorio
- **Comenta** con un breve resumen de las mejoras realizadas (en español)

## Reglas

- Nunca cambies el significado original del issue
- Si el issue ya está bien escrito, haz cambios mínimos
- Mantén el contenido útil, no verboso
- Todo el contenido debe estar en español
