package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrIntelligenceQuestionNotFound      = infraerrors.NotFound("INTELLIGENCE_QUESTION_NOT_FOUND", "intelligence question not found")
	ErrIntelligenceQuestionRequired      = infraerrors.BadRequest("INTELLIGENCE_QUESTION_REQUIRED", "question name, model, and prompt are required")
	ErrIntelligenceQuestionAnswerRequired = infraerrors.BadRequest("INTELLIGENCE_QUESTION_ANSWER_REQUIRED", "text questions require a reference answer")
)

type IntelligenceQuestion struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	ModelID         string    `json:"model_id"`
	QuestionText    string    `json:"question_text"`
	ExpectedAnswer  string    `json:"expected_answer"`
	ScoringCriteria string    `json:"scoring_criteria"`
	AnswerMode      string    `json:"answer_mode"`
	Strength        string    `json:"strength"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type IntelligenceQuestionInput struct {
	Name            string `json:"name"`
	ModelID         string `json:"model_id"`
	QuestionText    string `json:"question_text"`
	ExpectedAnswer  string `json:"expected_answer"`
	ScoringCriteria string `json:"scoring_criteria"`
	AnswerMode      string `json:"answer_mode"`
	Strength        string `json:"strength"`
	Enabled         *bool  `json:"enabled"`
}

type IntelligenceQuestionRepository interface {
	List(ctx context.Context, enabledOnly bool) ([]*IntelligenceQuestion, error)
	GetByID(ctx context.Context, id int64) (*IntelligenceQuestion, error)
	Create(ctx context.Context, question *IntelligenceQuestion) (*IntelligenceQuestion, error)
	Update(ctx context.Context, question *IntelligenceQuestion) (*IntelligenceQuestion, error)
	Delete(ctx context.Context, id int64) error
}

type IntelligenceQuestionService struct {
	repo IntelligenceQuestionRepository
}

func NewIntelligenceQuestionService(repo IntelligenceQuestionRepository) *IntelligenceQuestionService {
	return &IntelligenceQuestionService{repo: repo}
}

func (s *IntelligenceQuestionService) ListAll(ctx context.Context) ([]*IntelligenceQuestion, error) {
	return s.repo.List(ctx, false)
}

func (s *IntelligenceQuestionService) ListEnabled(ctx context.Context) ([]*IntelligenceQuestion, error) {
	return s.repo.List(ctx, true)
}

func (s *IntelligenceQuestionService) GetByID(ctx context.Context, id int64) (*IntelligenceQuestion, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *IntelligenceQuestionService) Create(ctx context.Context, input IntelligenceQuestionInput) (*IntelligenceQuestion, error) {
	question, err := normalizeIntelligenceQuestion(input)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, question)
}

func (s *IntelligenceQuestionService) Update(ctx context.Context, id int64, input IntelligenceQuestionInput) (*IntelligenceQuestion, error) {
	question, err := normalizeIntelligenceQuestion(input)
	if err != nil {
		return nil, err
	}
	question.ID = id
	return s.repo.Update(ctx, question)
}

func (s *IntelligenceQuestionService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *IntelligenceQuestionService) Score(question *IntelligenceQuestion, response string, upstreamSuccess bool) (float64, bool, string) {
	if question == nil {
		return 0, false, "question_not_found"
	}
	return evaluateIntelligenceAnswer(response, question.ExpectedAnswer, question.ScoringCriteria, question.AnswerMode, upstreamSuccess)
}

func normalizeIntelligenceQuestion(input IntelligenceQuestionInput) (*IntelligenceQuestion, error) {
	question := &IntelligenceQuestion{
		Name: strings.TrimSpace(input.Name), ModelID: strings.TrimSpace(input.ModelID),
		QuestionText: strings.TrimSpace(input.QuestionText), ExpectedAnswer: strings.TrimSpace(input.ExpectedAnswer),
		ScoringCriteria: strings.TrimSpace(input.ScoringCriteria), AnswerMode: input.AnswerMode,
		Strength: NormalizeIntelligenceStrength(input.Strength), Enabled: input.Enabled == nil || *input.Enabled,
	}
	if question.Name == "" || question.ModelID == "" || question.QuestionText == "" {
		return nil, ErrIntelligenceQuestionRequired
	}
	if question.AnswerMode != "svg" {
		question.AnswerMode = "text_exact"
	}
	if question.AnswerMode == "text_exact" && question.ExpectedAnswer == "" {
		return nil, ErrIntelligenceQuestionAnswerRequired
	}
	return question, nil
}

func NormalizeIntelligenceStrength(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "high":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "medium"
	}
}

func evaluateIntelligenceAnswer(response, expectedAnswer, _ string, answerMode string, upstreamSuccess bool) (float64, bool, string) {
	if !upstreamSuccess {
		return 0, false, "upstream_failed"
	}
	if answerMode == "svg" {
		if extractSVG(response) == "" {
			return 0, false, "svg_missing"
		}
		return 1, true, "svg_valid"
	}
	if strings.EqualFold(strings.TrimSpace(response), strings.TrimSpace(expectedAnswer)) {
		return 1, true, "exact_match"
	}
	return 0, false, "answer_mismatch"
}
