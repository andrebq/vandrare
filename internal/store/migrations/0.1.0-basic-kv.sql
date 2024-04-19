create table dt_key_value(
    item_key text not null,
    item_val blob not null,

    clk_updated_at_unixms integer not null,
    clk_trid integer not null,

    primary key (item_key)
);