SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: instance_state; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.instance_state AS ENUM (
    'PENDING',
    'CREATING',
    'RUNNING',
    'DELETING',
    'DELETED',
    'CREATION_FAILED'
);


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: blobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.blobs (
    hash character(16) NOT NULL,
    data bytea NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: chunks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chunks (
    id uuid NOT NULL,
    name character varying(50) NOT NULL,
    description character varying(100) NOT NULL,
    tags character varying(25)[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: flavor_version_files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.flavor_version_files (
    flavor_version_id uuid NOT NULL,
    file_hash character(16),
    file_path character varying(4096) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: flavor_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.flavor_versions (
    id uuid NOT NULL,
    flavor_id uuid NOT NULL,
    hash character(16) NOT NULL,
    change_hash character(16) NOT NULL,
    version character varying(25) NOT NULL,
    files_uploaded boolean DEFAULT false NOT NULL,
    prev_version_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: flavors; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.flavors (
    id uuid NOT NULL,
    chunk_id uuid NOT NULL,
    name character varying(25) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: instances; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.instances (
    id uuid NOT NULL,
    chunk_id uuid NOT NULL,
    flavor_id uuid NOT NULL,
    node_id uuid NOT NULL,
    port integer,
    state public.instance_state DEFAULT 'PENDING'::public.instance_state NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: nodes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.nodes (
    id uuid NOT NULL,
    address inet NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version character varying(128) NOT NULL
);


--
-- Name: blobs blobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.blobs
    ADD CONSTRAINT blobs_pkey PRIMARY KEY (hash);


--
-- Name: chunks chunks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chunks
    ADD CONSTRAINT chunks_pkey PRIMARY KEY (id);


--
-- Name: flavor_versions flavor_versions_flavor_id_version_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.flavor_versions
    ADD CONSTRAINT flavor_versions_flavor_id_version_key UNIQUE (flavor_id, version);


--
-- Name: flavor_versions flavor_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.flavor_versions
    ADD CONSTRAINT flavor_versions_pkey PRIMARY KEY (id);


--
-- Name: flavors flavors_id_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.flavors
    ADD CONSTRAINT flavors_id_name_key UNIQUE (id, name);


--
-- Name: flavors flavors_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.flavors
    ADD CONSTRAINT flavors_pkey PRIMARY KEY (id);


--
-- Name: instances instances_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.instances
    ADD CONSTRAINT instances_pkey PRIMARY KEY (id);


--
-- Name: nodes nodes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.nodes
    ADD CONSTRAINT nodes_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: flavor_hash_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX flavor_hash_idx ON public.flavor_versions USING btree (hash);


--
-- Name: flavor_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX flavor_name_idx ON public.flavors USING btree (name);


--
-- Name: flavor_version_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX flavor_version_idx ON public.flavor_versions USING btree (version);


--
-- Name: flavor_version_files flavor_version_files_flavor_version_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.flavor_version_files
    ADD CONSTRAINT flavor_version_files_flavor_version_id_fkey FOREIGN KEY (flavor_version_id) REFERENCES public.flavor_versions(id) ON DELETE CASCADE;


--
-- Name: flavor_versions flavor_versions_flavor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.flavor_versions
    ADD CONSTRAINT flavor_versions_flavor_id_fkey FOREIGN KEY (flavor_id) REFERENCES public.flavors(id) ON DELETE CASCADE;


--
-- Name: flavors flavors_chunk_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.flavors
    ADD CONSTRAINT flavors_chunk_id_fkey FOREIGN KEY (chunk_id) REFERENCES public.chunks(id);


--
-- Name: instances instances_chunk_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.instances
    ADD CONSTRAINT instances_chunk_id_fkey FOREIGN KEY (chunk_id) REFERENCES public.chunks(id);


--
-- Name: instances instances_flavor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.instances
    ADD CONSTRAINT instances_flavor_id_fkey FOREIGN KEY (flavor_id) REFERENCES public.flavors(id);


--
-- Name: instances instances_node_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.instances
    ADD CONSTRAINT instances_node_id_fkey FOREIGN KEY (node_id) REFERENCES public.nodes(id);


--
-- PostgreSQL database dump complete
--


--
-- Dbmate schema migrations
--

INSERT INTO public.schema_migrations (version) VALUES
    ('00000000000000');
