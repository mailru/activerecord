package repository

type Extra struct {
	Title   string
	Prepaid bool

	Other map[string]interface{} `mapstructure:",remain"`
}

type ServiceUnlocked struct {
	Android string
	IOS     string
	Web     string
	Huawei  string
}
