-- +goose Up
-- +goose StatementBegin

create table topkek (
    id serial not null primary key,
    name text not null,
    chat_id bigint not null,
    author_id bigint not null,
    created_at timestamp not null,
    status text not null 
); 

create table topkek_message (
    id serial not null primary key,
    topkek_id bigint not null,
    chat_id bigint not null,
    message_id bigint not null,
    type text not null
);

alter table topkek_message add constraint topkek_message_topkek_fk foreign key (topkek_id) references topkek(id) on delete restrict;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
select 1;
-- +goose StatementEnd