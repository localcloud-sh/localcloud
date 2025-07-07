--
-- PostgreSQL database dump
--

-- Dumped from database version 15.12 (Debian 15.12-1.pgdg120+1)
-- Dumped by pg_dump version 15.13 (Homebrew)

-- Started on 2025-07-06 15:58:09 +03

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- TOC entry 7 (class 2615 OID 24577)
-- Name: localcloud; Type: SCHEMA; Schema: -; Owner: localcloud
--

CREATE SCHEMA localcloud;


ALTER SCHEMA localcloud OWNER TO localcloud;

--
-- TOC entry 2 (class 3079 OID 24587)
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA localcloud;


--
-- TOC entry 3604 (class 0 OID 0)
-- Dependencies: 2
-- Name: EXTENSION vector; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION vector IS 'vector data type and ivfflat and hnsw access methods';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- TOC entry 217 (class 1259 OID 24915)
-- Name: embeddings; Type: TABLE; Schema: localcloud; Owner: localcloud
--

CREATE TABLE localcloud.embeddings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    document_id text NOT NULL,
    chunk_index integer NOT NULL,
    content text NOT NULL,
    embedding localcloud.vector(1536),
    metadata jsonb,
    created_at timestamp without time zone DEFAULT now()
);


ALTER TABLE localcloud.embeddings OWNER TO localcloud;

--
-- TOC entry 216 (class 1259 OID 24578)
-- Name: metadata; Type: TABLE; Schema: localcloud; Owner: localcloud
--

CREATE TABLE localcloud.metadata (
    key character varying(255) NOT NULL,
    value text,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE localcloud.metadata OWNER TO localcloud;

--
-- TOC entry 3598 (class 0 OID 24915)
-- Dependencies: 217
-- Data for Name: embeddings; Type: TABLE DATA; Schema: localcloud; Owner: localcloud
--

COPY localcloud.embeddings (id, document_id, chunk_index, content, embedding, metadata, created_at) FROM stdin;
\.


--
-- TOC entry 3597 (class 0 OID 24578)
-- Dependencies: 216
-- Data for Name: metadata; Type: TABLE DATA; Schema: localcloud; Owner: localcloud
--

COPY localcloud.metadata (key, value, created_at, updated_at) FROM stdin;
version	1.0.0	2025-07-06 12:56:32.226325	2025-07-06 12:56:32.226325
\.


--
-- TOC entry 3451 (class 2606 OID 24925)
-- Name: embeddings embeddings_document_id_chunk_index_key; Type: CONSTRAINT; Schema: localcloud; Owner: localcloud
--

ALTER TABLE ONLY localcloud.embeddings
    ADD CONSTRAINT embeddings_document_id_chunk_index_key UNIQUE (document_id, chunk_index);


--
-- TOC entry 3453 (class 2606 OID 24923)
-- Name: embeddings embeddings_pkey; Type: CONSTRAINT; Schema: localcloud; Owner: localcloud
--

ALTER TABLE ONLY localcloud.embeddings
    ADD CONSTRAINT embeddings_pkey PRIMARY KEY (id);


--
-- TOC entry 3449 (class 2606 OID 24586)
-- Name: metadata metadata_pkey; Type: CONSTRAINT; Schema: localcloud; Owner: localcloud
--

ALTER TABLE ONLY localcloud.metadata
    ADD CONSTRAINT metadata_pkey PRIMARY KEY (key);


--
-- TOC entry 3454 (class 1259 OID 24926)
-- Name: embeddings_vector_idx; Type: INDEX; Schema: localcloud; Owner: localcloud
--

CREATE INDEX embeddings_vector_idx ON localcloud.embeddings USING ivfflat (embedding localcloud.vector_cosine_ops) WITH (lists='100');


-- Completed on 2025-07-06 15:58:09 +03

--
-- PostgreSQL database dump complete
--

