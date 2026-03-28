---
on:
  issues:
    types: [opened, reopened]
  reaction: eyes
permissions:
      contents: read
      issues: read
      pull-requests: read
engine: copilot
network: defaults
tools:
  bash: false
  github:
    toolsets: [default]
safe-outputs:
  update-issue:
---

# Issue Enhancer

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

- Lee el README para entender el proyecto (DevRune configures AI development agents by resolving, fetching, and materializing packages of **skills**, **rules**, **MCP server definitions**, and **workflows** into your workspace)
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

