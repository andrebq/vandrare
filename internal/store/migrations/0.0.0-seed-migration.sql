create table if not exists t_migrations(
    ver_major integer not null,
    ver_minor integer not null,
    ver_patch integer not null,
    content text not null,
    filename text not null,
    checksum text not null,

    primary key(ver_major, ver_minor, ver_patch),
    unique (checksum)
);