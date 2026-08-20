package searchtemplates

// Store — персистентность шаблонов по username.
type Store interface {
	List(username string) ([]Template, error)
	ListAll() ([]TemplateWithAuthor, error)
	Create(username, name, query string) (Template, error)
	Update(username, id, name, query string) (Template, error)
	Delete(username, id string) error
}
