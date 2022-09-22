#!/bin/bash

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
	CREATE USER "go-work" WITH PASSWORD '$POSTGRES_APP_PASSWORD';
		CREATE TABLE public.jobs
    (
        id bigint NOT NULL GENERATED ALWAYS AS IDENTITY ( INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1 ),
        name character varying(255) COLLATE pg_catalog."default" NOT NULL,
        crontabstring character varying(50) COLLATE pg_catalog."default" NOT NULL,
        command character varying(512) COLLATE pg_catalog."default" NOT NULL,
        timeout bigint NOT NULL,
        nextexecutiontime timestamp with time zone,
        running boolean NOT NULL DEFAULT false,
        arguments character varying[] COLLATE pg_catalog."default",
        CONSTRAINT jobs_pkey PRIMARY KEY (id),
        CONSTRAINT unique_name UNIQUE (name)
    );
    ALTER TABLE IF EXISTS public.jobs
        OWNER to "$POSTGRES_USER";
    GRANT SELECT, UPDATE, INSERT, DELETE ON public.jobs TO "go-work";

    CREATE INDEX jobs_nextexecutiontime_idx
        ON public.jobs USING btree
        (nextexecutiontime ASC NULLS LAST)
EOSQL