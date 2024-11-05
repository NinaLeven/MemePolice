-- +goose Up
-- +goose StatementBegin

alter table topkek_message add column raw jsonb not null default '{}'::jsonb;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
select 1;
-- +goose StatementEnd