-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN current_agent_id TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN current_agent_id;
-- +goose StatementEnd
