create view vw_token_set as
    select
        token_id,
        salted_token,
        user,
        description,
        expires_at_unixms,

        case
        when (expires_at_unixms is null or expires_at_unixms > unixepoch('subsec')) then true
        else false
        end as is_active,

        clk_updated_at_unixms,
        clk_trid integer
    from dt_token_set