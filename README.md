# mundodolphins-stats

[![CI](https://github.com/Mundo-Dolphins/mundodolphins-stats/actions/workflows/ci.yml/badge.svg)](https://github.com/Mundo-Dolphins/mundodolphins-stats/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Librería Go (y CLI base) que genera el contenido estadístico de [Mundo Dolphins](https://mundodolphins.es)
a partir de **JSON ya procesado**: datos para Hugo, Markdown, asociación de vídeos de YouTube a partidos,
borradores editoriales y auditorías.

> Proyecto de la comunidad Mundo Dolphins. No está afiliado ni respaldado por la NFL, Miami Dolphins,
> Pro-Football-Reference ni YouTube; los nombres se usan solo de forma descriptiva. Ver [NOTICE](NOTICE).

## Qué incluye y qué no

La librería **empieza donde acaba la descarga**: lee los JSON versionados (temporadas, partidos,
caché de vídeos) y produce la salida. **No incluye ningún scraper**: ni descargadores, ni navegadores
headless, ni proxies, ni parsers del HTML de webs de terceros. Quien construya los JSON procesados es
responsable de tener derecho a usar sus fuentes.

| Incluido | Paquetes |
|---|---|
| Modelo de los datos procesados y sus *envelopes* versionados | `pfr`, `footballdb`, `nflcom`, `boxscore` (solo tipos) |
| Generación de JSON de página y datos para Hugo | `pagejson`, `hugodata` |
| Markdown, tablas e i18n (es) | `markdown/season_stats`, `markdown/tablefmt`, `gamemd`, `i18n` |
| Asociación de vídeos a partidos (API oficial de YouTube) | `videos`, `youtube`, `youtubeapi` |
| Artículos editoriales y auditoría | `articles`, `audit` |
| CLI base | `cli` |

### Puntos de extensión (los implementas tú)

| Interfaz | Para qué |
|---|---|
| `fetch.Fetcher` | Descargar una página (HTTP, navegador, proxy…). Se incluye un `MockFetcher` para tests |
| `assets.Downloader` | Guardar archivos remotos (logos). `hugodata.GenerateOptions.Assets` |
| `pagejson.GamebookParser` | Leer el PDF de un *game book*. `GameBuilder.SetGamebookParser` |
| `videos.Searcher`, `videos.WatchAvailabilityChecker`, `Service.Details` | Búsqueda y comprobación de vídeos fuera de la API oficial. `cli.SetVideoSearcher`, `cli.SetWatchChecker`, `cli.SetVideoDetailsFetcher` |
| `cli.Root()` | El comando raíz: añade tus propios subcomandos (`download`, `parse`…) en tu `main` |

```bash
go get github.com/Mundo-Dolphins/mundodolphins-stats@latest
```

> **Estabilidad:** mientras la versión sea `v0.x`, la API puede cambiar entre versiones menores.
> Fija una versión concreta en tu `go.mod`.

## Estructura de datos esperada

```text
processed/<año>.json                 temporada (pfr.Envelope[pfr.SeasonDataV1_1])
processed/games/<temporada>/<id>.json partido (boxscore.BoxscoreEnvelope)
processed/footballdb/…, processed/nflcom/…   datos complementarios (opcionales)
cache/videos/videos.json             caché de vídeos asociados a partidos
articles/<año>.md                    artículos editoriales
```

Salida: `output/data/historical/{seasons,season_stats,games,rivals}/…` y `output/historia/articulos`.

## CLI base

```bash
go run ./cmd/dolphins-stats --help
go run ./cmd/dolphins-stats generate hugo-data --help
```

El paquete `cli` está pensado para paquetes `main`: un programa propio puede añadir subcomandos a
`cli.Root()` antes de llamar a `cli.Execute()` (así se construye el binario completo con descarga y
parseo, que no forma parte de esta librería).

Comandos incluidos: `generate hugo-data`, `json season|season-stats|game|rivals`,
`videos discover|auto|backfill|review|web|…`, `articles missing|draft`, `audit season|status`.

## Desarrollo

```bash
make ci          # fmt-check + vet + test + lint
make coverage
```

Los tests no usan red real y sus fixtures son sintéticos o anonimizados ([docs/MAINTENANCE.md](docs/MAINTENANCE.md)).

## Seguridad

Ver [SECURITY.md](SECURITY.md).

## Licencia

[MIT](LICENSE)
