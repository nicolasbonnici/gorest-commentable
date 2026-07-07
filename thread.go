package commentable

import (
	"context"

	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/query"
)

// CommentThreadDTO is a comment enriched with its nested replies, forming the
// tree returned by the thread endpoint.
type CommentThreadDTO struct {
	CommentResponseDTO
	Children []*CommentThreadDTO `json:"children,omitempty"`
}

// fetchThread loads a full comment subtree for a commentable target.
//
// The gorest/query builder cannot express a portable recursive CTE (its
// recursive CTE helper has no UNION ALL self-reference and SelectBuilder has no
// UNION), so instead of walking the tree node by node (N+1 queries) it fetches
// one level at a time: every child of the previous level is loaded in a single
// IN-query. The walk is bounded to MaxNestingDepth levels, giving at most that
// many queries regardless of how many comments the thread contains.
func fetchThread(
	ctx context.Context,
	c *crud.CRUD[Comment],
	cfg *Config,
	commentableType, commentableID string,
	statusConds []query.Condition,
) ([]*CommentThreadDTO, error) {
	maxDepth := cfg.MaxNestingDepth
	if maxDepth < 1 {
		maxDepth = 1
	}

	rootConds := append([]query.Condition{
		query.Eq("commentable", commentableType),
		query.Eq("commentable_id", commentableID),
		query.IsNull("parent_id"),
	}, statusConds...)

	level, err := fetchLevel(ctx, c, rootConds)
	if err != nil {
		return nil, err
	}

	all := append([]Comment(nil), level...)

	for depth := 1; depth < maxDepth && len(level) > 0; depth++ {
		parentIDs := make([]any, len(level))
		for i := range level {
			parentIDs[i] = level[i].Id
		}

		childConds := append([]query.Condition{query.In("parent_id", parentIDs...)}, statusConds...)
		level, err = fetchLevel(ctx, c, childConds)
		if err != nil {
			return nil, err
		}
		all = append(all, level...)
	}

	return assembleTree(all), nil
}

func fetchLevel(ctx context.Context, c *crud.CRUD[Comment], conds []query.Condition) ([]Comment, error) {
	res, err := c.GetAllPaginated(ctx, crud.PaginationOptions{
		Conditions: conds,
		OrderBy:    []crud.OrderByClause{{Column: "created_at", Direction: query.ASC}},
	})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

func assembleTree(flat []Comment) []*CommentThreadDTO {
	conv := &CommentConverter{}
	nodes := make(map[string]*CommentThreadDTO, len(flat))
	for i := range flat {
		nodes[flat[i].Id] = &CommentThreadDTO{CommentResponseDTO: conv.ModelToResponseDTO(flat[i])}
	}

	roots := make([]*CommentThreadDTO, 0)
	for i := range flat {
		node := nodes[flat[i].Id]
		if flat[i].ParentId != nil {
			if parent, ok := nodes[*flat[i].ParentId]; ok {
				parent.Children = append(parent.Children, node)
				continue
			}
		}
		roots = append(roots, node)
	}
	return roots
}
