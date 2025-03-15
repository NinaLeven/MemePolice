-- +goose Up
-- +goose StatementBegin

create table chat_settings (
    chat_id bigint not null primary key,
    min_reactions int not null default 5,
    image_hamming_distance int not null default 3,
    video_hamming_distance int not null default 11
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
select 1;
-- +goose StatementEnd