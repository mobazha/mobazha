package events

type subSettings struct {
	buffer           int
	matchFieldValues map[string]string
}

var subSettingsDefault = subSettings{
	buffer: 16,
}

func BufSize(n int) func(interface{}) error {
	return func(s interface{}) error {
		s.(*subSettings).buffer = n
		return nil
	}
}

func MatchFields(fieldValueMap map[string]string) func(interface{}) error {
	return func(s interface{}) error {
		s.(*subSettings).matchFieldValues = fieldValueMap
		return nil
	}
}
