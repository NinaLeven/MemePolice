-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION bktree;

create table last_update_id (
    last_update_id int8 not null 
);

insert into last_update_id(last_update_id) values (599672059);

create sequence message_id_seq;

create table message (
    id bigint default nextval('message_id_seq'),
    chat_id bigint not null,
    message_id bigint not null,
    data jsonb not null,
    image_hash bigint,
    created_at timestamp not null,
    updated_at timestamp not null,
    primary key (id)
);

create unique index message_unique_idx on "message" using btree(chat_id, message_id);
CREATE INDEX bk_message_image_hash_idx ON message USING spgist (image_hash bktree_ops) where image_hash is not null;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
select 1;
-- +goose StatementEnd