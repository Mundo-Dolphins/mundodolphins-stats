package i18n

type Translator interface {
	T(key string) string
	TranslateScoringPlay(text string) string
	TranslateTeamName(name string) string
	TranslateMonth(month string) string
	TranslateDay(dayNum int) string
	TranslateWeather(text string) string
}
