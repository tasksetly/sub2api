package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type intelligenceQuestionRepository struct {
	db *sql.DB
}

func NewIntelligenceQuestionRepository(db *sql.DB) service.IntelligenceQuestionRepository {
	return &intelligenceQuestionRepository{db: db}
}

const intelligenceQuestionColumns = `id, name, model_id, question_text, expected_answer, scoring_criteria, answer_mode, strength, enabled, created_at, updated_at`

func (r *intelligenceQuestionRepository) List(ctx context.Context, enabledOnly bool) ([]*service.IntelligenceQuestion, error) {
	query := `SELECT ` + intelligenceQuestionColumns + ` FROM intelligence_test_questions`
	if enabledOnly {
		query += ` WHERE enabled = true`
	}
	query += ` ORDER BY updated_at DESC, id DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	questions := make([]*service.IntelligenceQuestion, 0)
	for rows.Next() {
		question := &service.IntelligenceQuestion{}
		if err := rows.Scan(&question.ID, &question.Name, &question.ModelID, &question.QuestionText, &question.ExpectedAnswer, &question.ScoringCriteria, &question.AnswerMode, &question.Strength, &question.Enabled, &question.CreatedAt, &question.UpdatedAt); err != nil {
			return nil, err
		}
		questions = append(questions, question)
	}
	return questions, rows.Err()
}

func (r *intelligenceQuestionRepository) GetByID(ctx context.Context, id int64) (*service.IntelligenceQuestion, error) {
	question := &service.IntelligenceQuestion{}
	err := r.db.QueryRowContext(ctx, `SELECT `+intelligenceQuestionColumns+` FROM intelligence_test_questions WHERE id = $1`, id).Scan(
		&question.ID, &question.Name, &question.ModelID, &question.QuestionText, &question.ExpectedAnswer, &question.ScoringCriteria,
		&question.AnswerMode, &question.Strength, &question.Enabled, &question.CreatedAt, &question.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, service.ErrIntelligenceQuestionNotFound
	}
	if err != nil {
		return nil, err
	}
	return question, nil
}

func (r *intelligenceQuestionRepository) Create(ctx context.Context, question *service.IntelligenceQuestion) (*service.IntelligenceQuestion, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO intelligence_test_questions (name, model_id, question_text, expected_answer, scoring_criteria, answer_mode, strength, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+intelligenceQuestionColumns,
		question.Name, question.ModelID, question.QuestionText, question.ExpectedAnswer, question.ScoringCriteria,
		question.AnswerMode, question.Strength, question.Enabled,
	)
	return scanIntelligenceQuestion(row)
}

func (r *intelligenceQuestionRepository) Update(ctx context.Context, question *service.IntelligenceQuestion) (*service.IntelligenceQuestion, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE intelligence_test_questions
		SET name = $2, model_id = $3, question_text = $4, expected_answer = $5, scoring_criteria = $6,
			answer_mode = $7, strength = $8, enabled = $9, updated_at = NOW()
		WHERE id = $1
		RETURNING `+intelligenceQuestionColumns,
		question.ID, question.Name, question.ModelID, question.QuestionText, question.ExpectedAnswer, question.ScoringCriteria,
		question.AnswerMode, question.Strength, question.Enabled,
	)
	return scanIntelligenceQuestion(row)
}

func (r *intelligenceQuestionRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM intelligence_test_questions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return service.ErrIntelligenceQuestionNotFound
	}
	return nil
}

type intelligenceQuestionRow interface {
	Scan(dest ...any) error
}

func scanIntelligenceQuestion(row intelligenceQuestionRow) (*service.IntelligenceQuestion, error) {
	question := &service.IntelligenceQuestion{}
	if err := row.Scan(&question.ID, &question.Name, &question.ModelID, &question.QuestionText, &question.ExpectedAnswer, &question.ScoringCriteria, &question.AnswerMode, &question.Strength, &question.Enabled, &question.CreatedAt, &question.UpdatedAt); err != nil {
		return nil, err
	}
	return question, nil
}
