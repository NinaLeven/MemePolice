-- +goose Up
-- +goose StatementBegin

alter table topkek add column message_id bigint;

UPDATE topkek
SET message_id = subquery.message_id
FROM (
    select m.chat_id, 
        m.message_id 
    from (
        SELECT max(id) as id
        FROM message
        GROUP BY chat_id
    ) AS last_message
    inner join message as m
        on m.id = last_message.id
) as subquery
WHERE topkek.chat_id = subquery.chat_id;

alter table topkek alter column message_id set not null;

create sequence message_reactions_id_seq;

create table message_reactions (
    id bigint default nextval('message_reactions_id_seq'),
    chat_id bigint not null,
    message_id bigint not null,
    user_id bigint not null,
    reactions jsonb not null,
    created_at timestamp not null default now(),
    updated_at timestamp not null default now()
);

create unique index message_reactions_unique_idx on "message_reactions" using btree(chat_id, message_id, user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
select 1;
-- +goose StatementEnd