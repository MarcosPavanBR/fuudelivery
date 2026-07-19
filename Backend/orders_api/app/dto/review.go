package dto

type CreateReviewRequest struct {
    OrderID     string `json:"order_id"`
    UserPhone   string `json:"user_phone"`
    UserName    string `json:"user_name"`
    ProductID   uint   `json:"product_id,omitempty"`
    Rating      int    `json:"rating"`
    Comment     string `json:"comment,omitempty"`
    ImageURL    string `json:"image_url,omitempty"`
    IsAnonymous bool   `json:"is_anonymous"`
}

type ReviewResponse struct {
    Rating    int    `json:"rating"`
    Comment   string `json:"comment"`
    UserName  string `json:"user_name,omitempty"`
    ImageURL  string `json:"image_url,omitempty"`
    CreatedAt string `json:"created_at"`
}

type RespondReviewRequest struct {
    ReviewID     uint   `json:"review_id"`
    ResponseText string `json:"response_text"`
}

type EstablishmentRating struct {
    EstablishmentID uint    `json:"establishment_id"`
    AverageRating   float64 `json:"average_rating"`
    TotalReviews    int     `json:"total_reviews"`
    RatingCounts    map[int]int `json:"rating_counts"`
}
