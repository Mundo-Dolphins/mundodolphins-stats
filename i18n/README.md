# i18n (Internationalization) Package

This package manages all translations for the mundodolphins-stats library.

## Structure

```
i18n/
├── translator.go           # Translator interface definition
├── es.go                   # Spanish translator implementation
├── loader.go               # YAML translations loader
├── translations/
│   └── es.yaml             # Spanish translations (centralized)
└── README.md               # This file
```

## Centralized Translation System

All translations are now centralized in **YAML files** (`translations/*.yaml`), making it easy to:

- ✅ Maintain translations in one place
- ✅ Avoid duplicating translations across the codebase
- ✅ Scale to multiple languages
- ✅ Edit translations without touching code

### Spanish Translations (es.yaml)

The `es.yaml` file contains all Spanish translations organized by category:

- **UI labels** - Basic UI strings (week, N/A, etc.)
- **Game info** - Game information labels (duration, attendance, etc.)
- **Officials** - Officials table header translations
- **Table legends** - Over 100+ table header translations organized by category:
  - Common terms
  - Snap counts
  - Drives
  - Kicking & punting
  - Expected points
  - Advanced statistics (passing, rushing, receiving, defense)
- **Months** - Month names in Spanish
- **Weather** - Weather pattern replacements
- **Scoring plays** - Scoring play pattern replacements

## Usage

### Loading Translations

```go
import "github.com/Mundo-Dolphins/phins-stats/internal/i18n"

// Load Spanish translations
translations, err := i18n.LoadTranslations("es")
if err != nil {
    // Handle error
}

// Access translations by category
if translation, ok := translations.GetGameInfoTranslation("won_toss"); ok {
    fmt.Println(translation)  // "Sorteo ganado por"
}

// For table legends
if translation, ok := translations.GetTableLegendTranslation("common", "player"); ok {
    fmt.Println(translation)  // "Jugador"
}

// Get all table legends as flat map
allLegends := translations.FlattenTableLegends()
```

### Using the Translator

```go
translator := i18n.NewESTranslator()

// Translate UI labels
label := translator.T("Week")  // "Semana"

// Translate months
month := translator.TranslateMonth("January")  // "enero"

// Translate weather descriptions
weather := translator.TranslateWeather("relative humidity")  // "humedad relativa"

// Translate scoring plays (complex patterns)
score := translator.TranslateScoringPlay("20 yards pass from Tua Tagovailoa")
```

## Adding New Translations

1. **Open** `internal/i18n/translations/es.yaml`
2. **Add** your translation under the appropriate category
3. **Example**:
```yaml
game_info:
  my_new_label: "Mi nueva etiqueta"

table_legends:
  common:
    my_new_key: "Mi nueva clave"
```
4. **No code changes needed** - translations are automatically loaded

## Future Language Support

To add support for another language:

1. **Create** `internal/i18n/translations/{language}.yaml` (e.g., `pt.yaml` for Portuguese)
2. **Populate** translations following the same structure as `es.yaml`
3. **Create** `internal/i18n/{language}.go` with appropriate translator implementation
4. **Load** with: `translations, _ := i18n.LoadTranslations("{language}")`

## Integration Points

### gamemd/builder.go
Uses:
- `i18n.Translations.GetGameInfoTranslation()` for game info labels
- `i18n.Translations.GetOfficialsTranslation()` for officials table headers
- `i18n.Translations.FlattenTableLegends()` for all table legends

### gamemd/builder.go
Manages translation of legends and game info using:
- `i18n.Translations.FlattenTableLegends()` for renderTable()
- `i18n.Translations.GetTableLegendTranslation()` for specific lookups
- Helper functions: `extractMainLegend()`, `stripHTMLTags()`, `splitLines()`

### i18n/es.go
Implements the `Translator` interface, supporting:
- Basic label translation
- Month translation
- Weather pattern translation
- Scoring play translation

## Embedded Files

Translations are embedded in the binary using Go's `embed` package, which means:
- ✅ No external files needed at runtime
- ✅ Binary is self-contained
- ✅ Translations loaded at compile time

## Best Practices

1. **Organize by category** - Keep related translations together
2. **Use consistent naming** - Use snake_case for keys
3. **Keep translations simple** - One key, one translation
4. **Document complex patterns** - Add comments for regex patterns
5. **Test translations** - Verify output with actual data

## Migration from Hardcoded Translations

All hardcoded translation dictionaries have been centralized to YAML:
- ✅ `internal/gamemd/builder.go` - Table legends + helper functions → `table_legends` in YAML
- ✅ `internal/gamemd/builder.go` - Game info labels → `game_info` in YAML  
- ✅ `internal/gamemd/builder.go` - Officials headers → `officials` in YAML
- ✅ `internal/i18n/es.go` - UI labels → `ui` in YAML

Note: `internal/gamemd/legends.go` was consolidated into `builder.go` for improved module organization.
