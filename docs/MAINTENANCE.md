# Rutina de mantenimiento

## Checks locales antes de abrir una PR

```bash
make ci        # fmt-check + vet + test + lint
make coverage
```

Los tests no usan red real: los servidores son `httptest`, las fuentes externas son *fakes* y los
fixtures son pequeños y sintéticos.

## Fixtures

Los ficheros de `testdata/` y `fixtures/` son **sintéticos o anonimizados**: contienen la estructura
de los datos procesados, pero nombres de personas sustituidos por alias (`Player A`…), tablas
reducidas a unas pocas filas y textos explicativos genéricos. No pegues datos reales de terceros en
los fixtures. Si cambias la estructura de la salida, regenera los golden files con:

```bash
GENERATE_GOLDEN=1 go test ./pagejson -run TestGolden_Generate
```

## Temporadas

Las reglas de temporada NFL de las rutas de partidos están en el paquete `util`
(`NFLSeasonFromGameID`): un partido de enero o febrero pertenece a la temporada anterior.

## Actualizar dependencias

Dependabot abre una PR semanal agrupada. A mano:

```bash
go get -u ./... && go mod tidy && make ci
```
