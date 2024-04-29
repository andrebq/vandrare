create table dt_token_set(
    token_id text not null,
    salted_token blob not null,
    user text not null,
    description text not null,
    expires_at_unixms integer,

    clk_updated_at_unixms integer not null,
    clk_trid integer not null,

    primary key(token_id)
);