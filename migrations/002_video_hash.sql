-- +goose Up
-- +goose StatementBegin
alter table message add column video_video_hash bigint default null;
alter table message add column video_audio_hash bigint default null;

CREATE INDEX bk_message_video_hash_idx ON message USING spgist (video_video_hash bktree_ops) where video_video_hash is not null;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
select 1;
-- +goose StatementEnd