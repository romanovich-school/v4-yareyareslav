package service

type Repository interface{}

type CatalogService struct {
	repository Repository
}

func NewCatalogService(repository Repository) *CatalogService {
	return &CatalogService{
		repository: repository,
	}
}
