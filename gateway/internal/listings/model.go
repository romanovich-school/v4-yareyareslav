package listings

type ListingID int

type Listing struct {
	ID        *ListingID
	Title     string
	Price     float64
	Stock     int
	Sold      int
	SellerID  int
	CreatedAt string
}
