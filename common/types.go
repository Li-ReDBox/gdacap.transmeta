package common

type Notification struct {
	Name     string
	Category string
	Comment  *string
	Tool     Tool
	Input    []Input
	Output   []Output
}

type Tool struct {
	Name    string
	Version string
}

type Input struct {
	Hash string
}

type Output struct {
	OriginalName string
	Hash         string
	Type         string
}
