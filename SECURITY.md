# Seguridad

## Reportar una vulnerabilidad

Usa **Security > Report a vulnerability** (aviso privado de GitHub) en este repositorio.
No abras un issue publico con detalles de una vulnerabilidad ni pegues tokens en issues o PRs.

## Buenas practicas al usar la libreria

- La clave de la API de YouTube (`YOUTUBE_API_KEY`) va **solo** en el entorno o en un `.env` fuera del repositorio. Evita `--youtube-api-key`: los argumentos son visibles en la lista de procesos.
- Si una clave se filtra, revocala en Google Cloud Console y genera otra con restriccion de API (solo YouTube Data API v3).
- La web de revision de videos (`videos web`) solo debe escucharse en `127.0.0.1`; rechaza peticiones con `Host` u `Origin` que no sean de la propia maquina.
