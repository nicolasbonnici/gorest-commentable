package commentable

import (
	"github.com/google/uuid"
)

type CommentConverter struct{}

func (c *CommentConverter) CreateDTOToModel(dto CommentCreateDTO) Comment {
	return Comment{
		Id:            uuid.New().String(),
		CommentableId: dto.CommentableId,
		Commentable:   dto.Commentable,
		ParentId:      dto.ParentId,
		Content:       dto.Content,
		Status:        StatusAwaiting,
	}
}

func (c *CommentConverter) UpdateDTOToModel(dto CommentUpdateDTO) Comment {
	comment := Comment{}
	if dto.Content != nil {
		comment.Content = *dto.Content
	}
	if dto.Status != nil {
		comment.Status = *dto.Status
	}
	return comment
}

func (c *CommentConverter) ModelToResponseDTO(model Comment) CommentResponseDTO {
	return CommentResponseDTO{
		ID:            model.Id,
		UserID:        model.UserId,
		CommentableID: model.CommentableId,
		Commentable:   model.Commentable,
		ParentID:      model.ParentId,
		Content:       model.Content,
		Status:        model.Status,
		IPAddress:     model.IpAddress,
		UserAgent:     model.UserAgent,
		UpdatedAt:     model.UpdatedAt,
		CreatedAt:     model.CreatedAt,
	}
}

func (c *CommentConverter) ModelsToResponseDTOs(models []Comment) []CommentResponseDTO {
	dtos := make([]CommentResponseDTO, len(models))
	for i, model := range models {
		dtos[i] = c.ModelToResponseDTO(model)
	}
	return dtos
}
