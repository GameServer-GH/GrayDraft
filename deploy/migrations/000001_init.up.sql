-- GrayDraft 初始库结构（基线 2026-10-28）。
-- 新环境执行本文件，再执行 000002_seed_edu.up.sql。

--
-- PostgreSQL database dump
--


-- Dumped from database version 16.14 (Debian 16.14-1.pgdg12+1)
-- Dumped by pg_dump version 16.14 (Debian 16.14-1.pgdg12+1)

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
-- Name: pg_trgm; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;


--
-- Name: EXTENSION pg_trgm; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pg_trgm IS 'text similarity measurement and index searching based on trigrams';


--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


--
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;


--
-- Name: EXTENSION vector; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION vector IS 'vector data type and ivfflat and hnsw access methods';


--
-- Name: set_updated_at(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.set_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: agent_blueprints; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_blueprints (
    blueprint_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id text NOT NULL,
    name text NOT NULL,
    version text NOT NULL,
    policy_version text DEFAULT 'edu_workspace_v1'::text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    tool_allowlist jsonb DEFAULT '[]'::jsonb NOT NULL,
    channel_config jsonb DEFAULT '{}'::jsonb NOT NULL,
    limits jsonb DEFAULT '{}'::jsonb NOT NULL,
    status text DEFAULT 'draft'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: agent_eval_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_eval_runs (
    eval_id uuid NOT NULL,
    tenant_id text DEFAULT ''::text NOT NULL,
    agent_id text DEFAULT ''::text NOT NULL,
    model text DEFAULT ''::text NOT NULL,
    policy_version text DEFAULT ''::text NOT NULL,
    dataset_path text DEFAULT ''::text NOT NULL,
    total integer DEFAULT 0 NOT NULL,
    passed integer DEFAULT 0 NOT NULL,
    pass_rate real DEFAULT 0 NOT NULL,
    report jsonb DEFAULT '{}'::jsonb NOT NULL,
    status text DEFAULT 'completed'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: agent_run_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_run_events (
    run_id uuid NOT NULL,
    tenant_id text NOT NULL,
    seq bigint NOT NULL,
    event_id uuid NOT NULL,
    type text NOT NULL,
    schema_version integer DEFAULT 1 NOT NULL,
    payload jsonb NOT NULL,
    occurred_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: agent_run_outbox; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_run_outbox (
    outbox_id bigint NOT NULL,
    tenant_id text NOT NULL,
    run_id uuid NOT NULL,
    event_seq bigint NOT NULL,
    topic text NOT NULL,
    payload jsonb NOT NULL,
    published_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: agent_run_outbox_outbox_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.agent_run_outbox_outbox_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: agent_run_outbox_outbox_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.agent_run_outbox_outbox_id_seq OWNED BY public.agent_run_outbox.outbox_id;


--
-- Name: agent_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_runs (
    run_id uuid NOT NULL,
    tenant_id text NOT NULL,
    actor_id text NOT NULL,
    agent_id text NOT NULL,
    conversation_id uuid,
    status text NOT NULL,
    policy_version text NOT NULL,
    step_count integer DEFAULT 0 NOT NULL,
    tool_call_count integer DEFAULT 0 NOT NULL,
    token_in bigint DEFAULT 0 NOT NULL,
    token_out bigint DEFAULT 0 NOT NULL,
    deadline_at timestamp with time zone NOT NULL,
    idempotency_key text NOT NULL,
    prompt bytea DEFAULT '\x'::bytea NOT NULL,
    authorized_kbs text[] DEFAULT '{}'::text[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: channel_bindings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.channel_bindings (
    binding_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id text NOT NULL,
    channel text NOT NULL,
    external_id text NOT NULL,
    blueprint_name text DEFAULT ''::text NOT NULL,
    blueprint_version text DEFAULT ''::text NOT NULL,
    agent_id text DEFAULT 'edu'::text NOT NULL,
    webhook_secret text DEFAULT ''::text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: conversation_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.conversation_messages (
    message_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id text NOT NULL,
    actor_id text NOT NULL,
    conversation_id uuid NOT NULL,
    run_id uuid,
    role text NOT NULL,
    content text NOT NULL,
    citations jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT conversation_messages_role_check CHECK ((role = ANY (ARRAY['system'::text, 'user'::text, 'assistant'::text, 'tool'::text])))
);


--
-- Name: conversation_summaries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.conversation_summaries (
    tenant_id text NOT NULL,
    conversation_id uuid NOT NULL,
    version integer NOT NULL,
    summary text NOT NULL,
    message_count integer DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: edu_edges; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.edu_edges (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    kb_type text NOT NULL,
    src_kind text NOT NULL,
    src_key text NOT NULL,
    rel text NOT NULL,
    dst_chunk_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: global_budget_policies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.global_budget_policies (
    tenant_id text NOT NULL,
    daily_token_limit bigint DEFAULT 0 NOT NULL,
    daily_run_limit integer DEFAULT 0 NOT NULL,
    collaborative_enabled boolean DEFAULT true NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: kb_catalog_version; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.kb_catalog_version (
    kb_type text NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    latest_doc_updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: kb_chunks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.kb_chunks (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    doc_id uuid NOT NULL,
    kb_type text NOT NULL,
    chunk_index integer DEFAULT 0 NOT NULL,
    content text NOT NULL,
    embedding public.vector(1024),
    quality_score real DEFAULT 0.5 NOT NULL,
    index_ready boolean DEFAULT true NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    search_text text,
    embedding_structured public.vector(1024),
    index_version integer DEFAULT 1 NOT NULL,
    chunker_version text DEFAULT '1'::text NOT NULL,
    chunk_position text DEFAULT ''::text NOT NULL,
    search_text_tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple'::regconfig, COALESCE(search_text, ''::text))) STORED
)
PARTITION BY LIST (kb_type);


--
-- Name: COLUMN kb_chunks.search_text; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.kb_chunks.search_text IS 'Augmented text for FTS/rerank: [文档标题] [主题] [正文]';


--
-- Name: COLUMN kb_chunks.embedding_structured; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.kb_chunks.embedding_structured IS '4.0: OCR structured KV / attributes vector';


--
-- Name: kb_chunks_default; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.kb_chunks_default (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    doc_id uuid NOT NULL,
    kb_type text NOT NULL,
    chunk_index integer DEFAULT 0 NOT NULL,
    content text NOT NULL,
    embedding public.vector(1024),
    quality_score real DEFAULT 0.5 NOT NULL,
    index_ready boolean DEFAULT true NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    search_text text,
    embedding_structured public.vector(1024),
    index_version integer DEFAULT 1 NOT NULL,
    chunker_version text DEFAULT '1'::text NOT NULL,
    chunk_position text DEFAULT ''::text NOT NULL,
    search_text_tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple'::regconfig, COALESCE(search_text, ''::text))) STORED
);


--
-- Name: kb_chunks_note; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.kb_chunks_note (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    doc_id uuid NOT NULL,
    kb_type text NOT NULL,
    chunk_index integer DEFAULT 0 NOT NULL,
    content text NOT NULL,
    embedding public.vector(1024),
    quality_score real DEFAULT 0.5 NOT NULL,
    index_ready boolean DEFAULT true NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    search_text text,
    embedding_structured public.vector(1024),
    index_version integer DEFAULT 1 NOT NULL,
    chunker_version text DEFAULT '1'::text NOT NULL,
    chunk_position text DEFAULT ''::text NOT NULL,
    search_text_tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple'::regconfig, COALESCE(search_text, ''::text))) STORED
);


--
-- Name: kb_chunks_paper; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.kb_chunks_paper (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    doc_id uuid NOT NULL,
    kb_type text NOT NULL,
    chunk_index integer DEFAULT 0 NOT NULL,
    content text NOT NULL,
    embedding public.vector(1024),
    quality_score real DEFAULT 0.5 NOT NULL,
    index_ready boolean DEFAULT true NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    search_text text,
    embedding_structured public.vector(1024),
    index_version integer DEFAULT 1 NOT NULL,
    chunker_version text DEFAULT '1'::text NOT NULL,
    chunk_position text DEFAULT ''::text NOT NULL,
    search_text_tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple'::regconfig, COALESCE(search_text, ''::text))) STORED
);


--
-- Name: kb_chunks_tutorial; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.kb_chunks_tutorial (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    doc_id uuid NOT NULL,
    kb_type text NOT NULL,
    chunk_index integer DEFAULT 0 NOT NULL,
    content text NOT NULL,
    embedding public.vector(1024),
    quality_score real DEFAULT 0.5 NOT NULL,
    index_ready boolean DEFAULT true NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    search_text text,
    embedding_structured public.vector(1024),
    index_version integer DEFAULT 1 NOT NULL,
    chunker_version text DEFAULT '1'::text NOT NULL,
    chunk_position text DEFAULT ''::text NOT NULL,
    search_text_tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple'::regconfig, COALESCE(search_text, ''::text))) STORED
);


--
-- Name: kb_doc_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.kb_doc_tags (
    doc_id uuid NOT NULL,
    kb_type text NOT NULL,
    tag text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: kb_documents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.kb_documents (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    kb_type text NOT NULL,
    title text,
    source_url text,
    content_hash text,
    kb_version bigint DEFAULT 1 NOT NULL,
    index_ready boolean DEFAULT true NOT NULL,
    quality_score real DEFAULT 0.5 NOT NULL,
    last_accessed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    extensions jsonb DEFAULT '{}'::jsonb NOT NULL,
    storage_tier text DEFAULT 'hot'::text NOT NULL,
    index_version integer DEFAULT 1 NOT NULL,
    CONSTRAINT kb_documents_quality_score_check CHECK (((quality_score >= (0)::double precision) AND (quality_score <= (1)::double precision))),
    CONSTRAINT kb_documents_storage_tier_check CHECK ((storage_tier = ANY (ARRAY['hot'::text, 'cold'::text])))
);


--
-- Name: COLUMN kb_documents.extensions; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.kb_documents.extensions IS 'Caller-defined metadata, e.g. article_id, article_type, geo lat/lng';


--
-- Name: COLUMN kb_documents.storage_tier; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.kb_documents.storage_tier IS 'O-18: hot=SSD检索优先; cold=长期未访问归档层';


--
-- Name: kb_global_index; Type: MATERIALIZED VIEW; Schema: public; Owner: -
--

CREATE MATERIALIZED VIEW public.kb_global_index AS
 SELECT c.id,
    c.doc_id,
    c.kb_type,
    "left"(c.content, 256) AS snippet,
    c.embedding,
    c.quality_score,
    d.title AS doc_title
   FROM (public.kb_chunks c
     JOIN public.kb_documents d ON ((d.id = c.doc_id)))
  WHERE ((c.index_ready = true) AND (d.index_ready = true) AND (d.storage_tier = 'hot'::text))
  WITH NO DATA;


--
-- Name: kb_qa_pairs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.kb_qa_pairs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    doc_id uuid NOT NULL,
    kb_type text NOT NULL,
    chunk_id uuid,
    question text NOT NULL,
    answer text NOT NULL,
    quality_score real DEFAULT 0.5 NOT NULL,
    index_ready boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    index_version integer DEFAULT 1 NOT NULL
)
PARTITION BY LIST (kb_type);


--
-- Name: kb_qa_pairs_default; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.kb_qa_pairs_default (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    doc_id uuid NOT NULL,
    kb_type text NOT NULL,
    chunk_id uuid,
    question text NOT NULL,
    answer text NOT NULL,
    quality_score real DEFAULT 0.5 NOT NULL,
    index_ready boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    index_version integer DEFAULT 1 NOT NULL
);


--
-- Name: kb_types; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.kb_types (
    name text NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    context_profile jsonb
);


--
-- Name: COLUMN kb_types.context_profile; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.kb_types.context_profile IS 'Workspace background: description, entities[], rules[], negative_context[]';


--
-- Name: knowledge_bases; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_bases (
    id text NOT NULL,
    tenant_id text NOT NULL,
    display_name text,
    description text,
    storage_partition text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE knowledge_bases; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.knowledge_bases IS 'Knowledge base under a tenant; storage_partition is the pg kb_type partition key';


--
-- Name: COLUMN knowledge_bases.storage_partition; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.knowledge_bases.storage_partition IS 'Maps to kb_types.name for documents/chunks partitions';


--
-- Name: memory_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.memory_items (
    memory_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id text NOT NULL,
    actor_id text NOT NULL,
    kind text NOT NULL,
    subject text NOT NULL,
    value jsonb NOT NULL,
    source_message_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    confidence real DEFAULT 0 NOT NULL,
    sensitivity text DEFAULT 'normal'::text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    expires_at timestamp with time zone,
    embedding public.vector(1536),
    version integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT memory_items_kind_check CHECK ((kind = ANY (ARRAY['preference'::text, 'stable_fact'::text, 'task_state'::text, 'feedback'::text]))),
    CONSTRAINT memory_items_status_check CHECK ((status = ANY (ARRAY['active'::text, 'conflict'::text, 'deleted'::text])))
);


--
-- Name: model_cost_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.model_cost_events (
    id bigint NOT NULL,
    trace_id text,
    task_id uuid,
    session_id text,
    workspace text DEFAULT ''::text NOT NULL,
    kb_type text,
    api_key_id text,
    provider text DEFAULT ''::text NOT NULL,
    model text DEFAULT ''::text NOT NULL,
    operation text NOT NULL,
    input_tokens bigint DEFAULT 0,
    output_tokens bigint DEFAULT 0,
    item_count integer DEFAULT 0,
    latency_ms integer,
    status text DEFAULT 'ok'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    tenant_id text DEFAULT ''::text NOT NULL,
    agent_id text DEFAULT ''::text NOT NULL,
    run_id uuid
);


--
-- Name: model_cost_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.model_cost_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: model_cost_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.model_cost_events_id_seq OWNED BY public.model_cost_events.id;


--
-- Name: multimodal_content_cache; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.multimodal_content_cache (
    content_md5 text NOT NULL,
    mime_type text DEFAULT ''::text NOT NULL,
    route text DEFAULT 'vlm'::text NOT NULL,
    extracted_text text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_used_at timestamp with time zone DEFAULT now() NOT NULL,
    phash text,
    structured_kv text
);


--
-- Name: TABLE multimodal_content_cache; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.multimodal_content_cache IS '4.0: cached VLM/OCR extraction by file MD5';


--
-- Name: COLUMN multimodal_content_cache.phash; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.multimodal_content_cache.phash IS '4.0: perceptual hash for near-duplicate images';


--
-- Name: COLUMN multimodal_content_cache.structured_kv; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.multimodal_content_cache.structured_kv IS '4.0: flattened key:value text for structured embedding';


--
-- Name: ragops_chunk_strikes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ragops_chunk_strikes (
    chunk_id uuid NOT NULL,
    kb_type text NOT NULL,
    strike_date date DEFAULT CURRENT_DATE NOT NULL,
    source text DEFAULT 'feedback'::text NOT NULL,
    confidence real
);


--
-- Name: ragops_heal_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ragops_heal_jobs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    kb_type text NOT NULL,
    doc_id uuid NOT NULL,
    chunk_id uuid,
    reason text NOT NULL,
    action text DEFAULT 'rechunk'::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    strike_count integer DEFAULT 1 NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: tenant_tool_bindings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_tool_bindings (
    tenant_id text NOT NULL,
    tool_name text NOT NULL,
    tool_version text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    scopes jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: tenants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenants (
    id text NOT NULL,
    display_name text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE tenants; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.tenants IS 'Logical tenant (app/workspace); independent of GrayDraft';


--
-- Name: tool_approvals; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tool_approvals (
    approval_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id text NOT NULL,
    actor_id text NOT NULL,
    run_id uuid NOT NULL,
    call_id text NOT NULL,
    tool_name text NOT NULL,
    arguments_hash text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    decided_at timestamp with time zone,
    decided_by text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tool_approvals_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'approved'::text, 'rejected'::text, 'expired'::text])))
);


--
-- Name: tool_invocations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tool_invocations (
    invocation_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id text NOT NULL,
    run_id uuid,
    call_id text,
    tool_name text NOT NULL,
    risk_level text NOT NULL,
    status text NOT NULL,
    arguments_hash text NOT NULL,
    output_hash text,
    latency_ms integer,
    error_code text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: tool_specs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tool_specs (
    name text NOT NULL,
    version text NOT NULL,
    risk_level text DEFAULT 'R0'::text NOT NULL,
    scopes jsonb DEFAULT '[]'::jsonb NOT NULL,
    required_args jsonb DEFAULT '[]'::jsonb NOT NULL,
    schema_hash text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'draft'::text NOT NULL,
    timeout_ms integer DEFAULT 1500 NOT NULL,
    idempotent boolean DEFAULT true NOT NULL,
    spec jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    runner_type text DEFAULT 'in_process'::text NOT NULL,
    runner_command text,
    network_allowlist jsonb DEFAULT '[]'::jsonb NOT NULL,
    secret_refs jsonb DEFAULT '[]'::jsonb NOT NULL,
    CONSTRAINT tool_specs_status_check CHECK ((status = ANY (ARRAY['draft'::text, 'active'::text, 'deprecated'::text, 'disabled'::text])))
);


--
-- Name: ugc_ingest_stage_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ugc_ingest_stage_events (
    id bigint NOT NULL,
    task_id uuid NOT NULL,
    trace_id text,
    workspace text DEFAULT ''::text NOT NULL,
    stage text NOT NULL,
    status text NOT NULL,
    attempt integer DEFAULT 1 NOT NULL,
    index_version integer,
    chunker_version text,
    error_code text,
    error_message text,
    queued_at timestamp with time zone,
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: ugc_ingest_stage_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ugc_ingest_stage_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ugc_ingest_stage_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ugc_ingest_stage_events_id_seq OWNED BY public.ugc_ingest_stage_events.id;


--
-- Name: ugc_ingest_tasks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ugc_ingest_tasks (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    doc_id uuid,
    kb_type text NOT NULL,
    title text,
    status text DEFAULT 'pending'::text NOT NULL,
    error_message text,
    chunk_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    current_stage text,
    stage_retry_count integer DEFAULT 0 NOT NULL,
    ingest_deadline_at timestamp with time zone,
    ready_at timestamp with time zone,
    CONSTRAINT ugc_ingest_tasks_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'processing'::text, 'completed'::text, 'failed'::text])))
);


--
-- Name: workspace_4d_taxonomy; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.workspace_4d_taxonomy (
    kb_type text NOT NULL,
    domain text DEFAULT 'default'::text NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    scenes jsonb DEFAULT '[]'::jsonb NOT NULL,
    styles jsonb DEFAULT '[]'::jsonb NOT NULL,
    candidates jsonb DEFAULT '[]'::jsonb NOT NULL,
    stats jsonb DEFAULT '{}'::jsonb NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE workspace_4d_taxonomy; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.workspace_4d_taxonomy IS 'Workspace-scoped 4D keyword overlay; candidates promoted after doc_count threshold';


--
-- Name: kb_chunks_default; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_chunks ATTACH PARTITION public.kb_chunks_default DEFAULT;


--
-- Name: kb_chunks_note; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_chunks ATTACH PARTITION public.kb_chunks_note FOR VALUES IN ('note');


--
-- Name: kb_chunks_paper; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_chunks ATTACH PARTITION public.kb_chunks_paper FOR VALUES IN ('paper');


--
-- Name: kb_chunks_tutorial; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_chunks ATTACH PARTITION public.kb_chunks_tutorial FOR VALUES IN ('tutorial');


--
-- Name: kb_qa_pairs_default; Type: TABLE ATTACH; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_qa_pairs ATTACH PARTITION public.kb_qa_pairs_default DEFAULT;


--
-- Name: agent_run_outbox outbox_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_run_outbox ALTER COLUMN outbox_id SET DEFAULT nextval('public.agent_run_outbox_outbox_id_seq'::regclass);


--
-- Name: model_cost_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.model_cost_events ALTER COLUMN id SET DEFAULT nextval('public.model_cost_events_id_seq'::regclass);


--
-- Name: ugc_ingest_stage_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ugc_ingest_stage_events ALTER COLUMN id SET DEFAULT nextval('public.ugc_ingest_stage_events_id_seq'::regclass);


--
-- Name: agent_blueprints agent_blueprints_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_blueprints
    ADD CONSTRAINT agent_blueprints_pkey PRIMARY KEY (blueprint_id);


--
-- Name: agent_blueprints agent_blueprints_tenant_id_name_version_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_blueprints
    ADD CONSTRAINT agent_blueprints_tenant_id_name_version_key UNIQUE (tenant_id, name, version);


--
-- Name: agent_eval_runs agent_eval_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_eval_runs
    ADD CONSTRAINT agent_eval_runs_pkey PRIMARY KEY (eval_id);


--
-- Name: agent_run_events agent_run_events_event_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_run_events
    ADD CONSTRAINT agent_run_events_event_id_key UNIQUE (event_id);


--
-- Name: agent_run_events agent_run_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_run_events
    ADD CONSTRAINT agent_run_events_pkey PRIMARY KEY (run_id, seq);


--
-- Name: agent_run_outbox agent_run_outbox_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_run_outbox
    ADD CONSTRAINT agent_run_outbox_pkey PRIMARY KEY (outbox_id);


--
-- Name: agent_run_outbox agent_run_outbox_run_id_event_seq_topic_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_run_outbox
    ADD CONSTRAINT agent_run_outbox_run_id_event_seq_topic_key UNIQUE (run_id, event_seq, topic);


--
-- Name: agent_runs agent_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_runs
    ADD CONSTRAINT agent_runs_pkey PRIMARY KEY (run_id);


--
-- Name: agent_runs agent_runs_tenant_id_idempotency_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_runs
    ADD CONSTRAINT agent_runs_tenant_id_idempotency_key_key UNIQUE (tenant_id, idempotency_key);


--
-- Name: channel_bindings channel_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.channel_bindings
    ADD CONSTRAINT channel_bindings_pkey PRIMARY KEY (binding_id);


--
-- Name: channel_bindings channel_bindings_tenant_id_channel_external_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.channel_bindings
    ADD CONSTRAINT channel_bindings_tenant_id_channel_external_id_key UNIQUE (tenant_id, channel, external_id);


--
-- Name: conversation_messages conversation_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversation_messages
    ADD CONSTRAINT conversation_messages_pkey PRIMARY KEY (message_id);


--
-- Name: conversation_summaries conversation_summaries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversation_summaries
    ADD CONSTRAINT conversation_summaries_pkey PRIMARY KEY (tenant_id, conversation_id, version);


--
-- Name: edu_edges edu_edges_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edu_edges
    ADD CONSTRAINT edu_edges_pkey PRIMARY KEY (id);


--
-- Name: global_budget_policies global_budget_policies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.global_budget_policies
    ADD CONSTRAINT global_budget_policies_pkey PRIMARY KEY (tenant_id);


--
-- Name: kb_catalog_version kb_catalog_version_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_catalog_version
    ADD CONSTRAINT kb_catalog_version_pkey PRIMARY KEY (kb_type);


--
-- Name: kb_chunks kb_chunks_doc_id_chunk_index_kb_type_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_chunks
    ADD CONSTRAINT kb_chunks_doc_id_chunk_index_kb_type_key UNIQUE (doc_id, chunk_index, kb_type);


--
-- Name: kb_chunks_default kb_chunks_default_doc_id_chunk_index_kb_type_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_chunks_default
    ADD CONSTRAINT kb_chunks_default_doc_id_chunk_index_kb_type_key UNIQUE (doc_id, chunk_index, kb_type);


--
-- Name: kb_chunks kb_chunks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_chunks
    ADD CONSTRAINT kb_chunks_pkey PRIMARY KEY (id, kb_type);


--
-- Name: kb_chunks_default kb_chunks_default_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_chunks_default
    ADD CONSTRAINT kb_chunks_default_pkey PRIMARY KEY (id, kb_type);


--
-- Name: kb_chunks_note kb_chunks_note_doc_id_chunk_index_kb_type_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_chunks_note
    ADD CONSTRAINT kb_chunks_note_doc_id_chunk_index_kb_type_key UNIQUE (doc_id, chunk_index, kb_type);


--
-- Name: kb_chunks_note kb_chunks_note_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_chunks_note
    ADD CONSTRAINT kb_chunks_note_pkey PRIMARY KEY (id, kb_type);


--
-- Name: kb_chunks_paper kb_chunks_paper_doc_id_chunk_index_kb_type_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_chunks_paper
    ADD CONSTRAINT kb_chunks_paper_doc_id_chunk_index_kb_type_key UNIQUE (doc_id, chunk_index, kb_type);


--
-- Name: kb_chunks_paper kb_chunks_paper_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_chunks_paper
    ADD CONSTRAINT kb_chunks_paper_pkey PRIMARY KEY (id, kb_type);


--
-- Name: kb_chunks_tutorial kb_chunks_tutorial_doc_id_chunk_index_kb_type_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_chunks_tutorial
    ADD CONSTRAINT kb_chunks_tutorial_doc_id_chunk_index_kb_type_key UNIQUE (doc_id, chunk_index, kb_type);


--
-- Name: kb_chunks_tutorial kb_chunks_tutorial_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_chunks_tutorial
    ADD CONSTRAINT kb_chunks_tutorial_pkey PRIMARY KEY (id, kb_type);


--
-- Name: kb_doc_tags kb_doc_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_doc_tags
    ADD CONSTRAINT kb_doc_tags_pkey PRIMARY KEY (doc_id, tag);


--
-- Name: kb_documents kb_documents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_documents
    ADD CONSTRAINT kb_documents_pkey PRIMARY KEY (id);


--
-- Name: kb_qa_pairs kb_qa_pairs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_qa_pairs
    ADD CONSTRAINT kb_qa_pairs_pkey PRIMARY KEY (id, kb_type);


--
-- Name: kb_qa_pairs_default kb_qa_pairs_default_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_qa_pairs_default
    ADD CONSTRAINT kb_qa_pairs_default_pkey PRIMARY KEY (id, kb_type);


--
-- Name: kb_types kb_types_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_types
    ADD CONSTRAINT kb_types_pkey PRIMARY KEY (name);


--
-- Name: knowledge_bases knowledge_bases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_bases
    ADD CONSTRAINT knowledge_bases_pkey PRIMARY KEY (tenant_id, id);


--
-- Name: memory_items memory_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.memory_items
    ADD CONSTRAINT memory_items_pkey PRIMARY KEY (memory_id);


--
-- Name: model_cost_events model_cost_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.model_cost_events
    ADD CONSTRAINT model_cost_events_pkey PRIMARY KEY (id);


--
-- Name: multimodal_content_cache multimodal_content_cache_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.multimodal_content_cache
    ADD CONSTRAINT multimodal_content_cache_pkey PRIMARY KEY (content_md5);


--
-- Name: ragops_chunk_strikes ragops_chunk_strikes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ragops_chunk_strikes
    ADD CONSTRAINT ragops_chunk_strikes_pkey PRIMARY KEY (chunk_id, kb_type, strike_date, source);


--
-- Name: ragops_heal_jobs ragops_heal_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ragops_heal_jobs
    ADD CONSTRAINT ragops_heal_jobs_pkey PRIMARY KEY (id);


--
-- Name: tenant_tool_bindings tenant_tool_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_tool_bindings
    ADD CONSTRAINT tenant_tool_bindings_pkey PRIMARY KEY (tenant_id, tool_name);


--
-- Name: tenants tenants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_pkey PRIMARY KEY (id);


--
-- Name: tool_approvals tool_approvals_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tool_approvals
    ADD CONSTRAINT tool_approvals_pkey PRIMARY KEY (approval_id);


--
-- Name: tool_approvals tool_approvals_run_id_call_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tool_approvals
    ADD CONSTRAINT tool_approvals_run_id_call_id_key UNIQUE (run_id, call_id);


--
-- Name: tool_invocations tool_invocations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tool_invocations
    ADD CONSTRAINT tool_invocations_pkey PRIMARY KEY (invocation_id);


--
-- Name: tool_specs tool_specs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tool_specs
    ADD CONSTRAINT tool_specs_pkey PRIMARY KEY (name, version);


--
-- Name: ugc_ingest_stage_events ugc_ingest_stage_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ugc_ingest_stage_events
    ADD CONSTRAINT ugc_ingest_stage_events_pkey PRIMARY KEY (id);


--
-- Name: ugc_ingest_tasks ugc_ingest_tasks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ugc_ingest_tasks
    ADD CONSTRAINT ugc_ingest_tasks_pkey PRIMARY KEY (id);


--
-- Name: workspace_4d_taxonomy workspace_4d_taxonomy_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workspace_4d_taxonomy
    ADD CONSTRAINT workspace_4d_taxonomy_pkey PRIMARY KEY (kb_type);


--
-- Name: idx_agent_eval_runs_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_eval_runs_created ON public.agent_eval_runs USING btree (created_at DESC);


--
-- Name: idx_agent_eval_runs_tenant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_eval_runs_tenant ON public.agent_eval_runs USING btree (tenant_id, created_at DESC);


--
-- Name: idx_agent_run_events_tenant_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_run_events_tenant_time ON public.agent_run_events USING btree (tenant_id, occurred_at DESC);


--
-- Name: idx_agent_runs_tenant_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_runs_tenant_created ON public.agent_runs USING btree (tenant_id, created_at DESC);


--
-- Name: idx_chunks_search_text_gin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chunks_search_text_gin ON ONLY public.kb_chunks USING gin (search_text_tsv);


--
-- Name: idx_conversation_messages_recent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_conversation_messages_recent ON public.conversation_messages USING btree (tenant_id, conversation_id, created_at DESC);


--
-- Name: idx_cost_events_agent_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cost_events_agent_created ON public.model_cost_events USING btree (agent_id, created_at DESC);


--
-- Name: idx_cost_events_operation; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cost_events_operation ON public.model_cost_events USING btree (operation, created_at DESC);


--
-- Name: idx_cost_events_tenant_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cost_events_tenant_created ON public.model_cost_events USING btree (tenant_id, created_at DESC);


--
-- Name: idx_cost_events_workspace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cost_events_workspace ON public.model_cost_events USING btree (workspace, created_at DESC);


--
-- Name: idx_edu_edges_lookup; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_edu_edges_lookup ON public.edu_edges USING btree (kb_type, src_key);


--
-- Name: idx_kb_chunks_content_fts; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_chunks_content_fts ON ONLY public.kb_chunks USING gin (to_tsvector('simple'::regconfig, content));


--
-- Name: idx_kb_chunks_content_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_chunks_content_trgm ON ONLY public.kb_chunks USING gin (content public.gin_trgm_ops);


--
-- Name: idx_kb_chunks_embedding_hnsw; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_chunks_embedding_hnsw ON ONLY public.kb_chunks USING hnsw (embedding public.vector_cosine_ops) WITH (m='16', ef_construction='64') WHERE ((index_ready = true) AND (embedding IS NOT NULL));


--
-- Name: idx_kb_chunks_embedding_structured_hnsw; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_chunks_embedding_structured_hnsw ON ONLY public.kb_chunks USING hnsw (embedding_structured public.vector_cosine_ops) WITH (m='16', ef_construction='64') WHERE ((index_ready = true) AND (embedding_structured IS NOT NULL));


--
-- Name: idx_kb_chunks_ready_kb_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_chunks_ready_kb_type ON ONLY public.kb_chunks USING btree (kb_type, index_ready) WHERE (index_ready = true);


--
-- Name: idx_kb_chunks_search_text_fts; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_chunks_search_text_fts ON ONLY public.kb_chunks USING gin (to_tsvector('simple'::regconfig, COALESCE(NULLIF(search_text, ''::text), content)));


--
-- Name: idx_kb_chunks_search_text_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_chunks_search_text_trgm ON ONLY public.kb_chunks USING gin (COALESCE(NULLIF(search_text, ''::text), content) public.gin_trgm_ops);


--
-- Name: idx_kb_doc_tags_kb_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_doc_tags_kb_type ON public.kb_doc_tags USING btree (kb_type, tag);


--
-- Name: idx_kb_documents_extensions_gin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_documents_extensions_gin ON public.kb_documents USING gin (extensions);


--
-- Name: idx_kb_documents_kb_type_ready; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_documents_kb_type_ready ON public.kb_documents USING btree (kb_type, index_ready) WHERE (index_ready = true);


--
-- Name: idx_kb_documents_tier_access; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_documents_tier_access ON public.kb_documents USING btree (storage_tier, last_accessed_at DESC NULLS LAST);


--
-- Name: idx_kb_global_index_fts; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_global_index_fts ON public.kb_global_index USING gin (to_tsvector('simple'::regconfig, snippet));


--
-- Name: idx_kb_global_index_hnsw; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_global_index_hnsw ON public.kb_global_index USING hnsw (embedding public.vector_cosine_ops) WHERE (embedding IS NOT NULL);


--
-- Name: idx_kb_global_index_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_kb_global_index_id ON public.kb_global_index USING btree (id, kb_type);


--
-- Name: idx_kb_qa_pairs_doc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_qa_pairs_doc ON ONLY public.kb_qa_pairs USING btree (doc_id, kb_type);


--
-- Name: idx_kb_types_context_profile_gin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_types_context_profile_gin ON public.kb_types USING gin (context_profile);


--
-- Name: idx_knowledge_bases_storage; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_bases_storage ON public.knowledge_bases USING btree (storage_partition);


--
-- Name: idx_memory_items_relation; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_memory_items_relation ON public.memory_items USING btree (tenant_id, actor_id, kind, subject, status);


--
-- Name: idx_memory_items_vector; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_memory_items_vector ON public.memory_items USING ivfflat (embedding public.vector_cosine_ops) WITH (lists='100');


--
-- Name: idx_multimodal_cache_last_used; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_multimodal_cache_last_used ON public.multimodal_content_cache USING btree (last_used_at DESC);


--
-- Name: idx_multimodal_cache_phash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_multimodal_cache_phash ON public.multimodal_content_cache USING btree (phash) WHERE ((phash IS NOT NULL) AND (phash <> ''::text));


--
-- Name: idx_ragops_chunk_strikes_recent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ragops_chunk_strikes_recent ON public.ragops_chunk_strikes USING btree (kb_type, strike_date DESC);


--
-- Name: idx_ragops_heal_jobs_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ragops_heal_jobs_status ON public.ragops_heal_jobs USING btree (status, created_at DESC);


--
-- Name: idx_stage_events_task; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_stage_events_task ON public.ugc_ingest_stage_events USING btree (task_id, stage, attempt);


--
-- Name: idx_stage_events_workspace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_stage_events_workspace ON public.ugc_ingest_stage_events USING btree (workspace, created_at DESC);


--
-- Name: idx_tool_approvals_expiry; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tool_approvals_expiry ON public.tool_approvals USING btree (status, expires_at);


--
-- Name: idx_tool_invocations_tenant_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tool_invocations_tenant_time ON public.tool_invocations USING btree (tenant_id, created_at DESC);


--
-- Name: idx_tool_specs_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tool_specs_active ON public.tool_specs USING btree (name, status, updated_at DESC);


--
-- Name: idx_ugc_tasks_kb_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ugc_tasks_kb_type ON public.ugc_ingest_tasks USING btree (kb_type, created_at DESC);


--
-- Name: idx_ugc_tasks_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ugc_tasks_status ON public.ugc_ingest_tasks USING btree (status, created_at DESC);


--
-- Name: idx_workspace_4d_taxonomy_updated; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_workspace_4d_taxonomy_updated ON public.workspace_4d_taxonomy USING btree (updated_at DESC);


--
-- Name: kb_chunks_default_coalesce_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_default_coalesce_idx ON public.kb_chunks_default USING gin (COALESCE(NULLIF(search_text, ''::text), content) public.gin_trgm_ops);


--
-- Name: kb_chunks_default_content_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_default_content_idx ON public.kb_chunks_default USING gin (content public.gin_trgm_ops);


--
-- Name: kb_chunks_default_embedding_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_default_embedding_idx ON public.kb_chunks_default USING hnsw (embedding public.vector_cosine_ops) WITH (m='16', ef_construction='64') WHERE ((index_ready = true) AND (embedding IS NOT NULL));


--
-- Name: kb_chunks_default_embedding_structured_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_default_embedding_structured_idx ON public.kb_chunks_default USING hnsw (embedding_structured public.vector_cosine_ops) WITH (m='16', ef_construction='64') WHERE ((index_ready = true) AND (embedding_structured IS NOT NULL));


--
-- Name: kb_chunks_default_kb_type_index_ready_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_default_kb_type_index_ready_idx ON public.kb_chunks_default USING btree (kb_type, index_ready) WHERE (index_ready = true);


--
-- Name: kb_chunks_default_search_text_tsv_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_default_search_text_tsv_idx ON public.kb_chunks_default USING gin (search_text_tsv);


--
-- Name: kb_chunks_default_to_tsvector_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_default_to_tsvector_idx ON public.kb_chunks_default USING gin (to_tsvector('simple'::regconfig, content));


--
-- Name: kb_chunks_default_to_tsvector_idx1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_default_to_tsvector_idx1 ON public.kb_chunks_default USING gin (to_tsvector('simple'::regconfig, COALESCE(NULLIF(search_text, ''::text), content)));


--
-- Name: kb_chunks_note_coalesce_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_note_coalesce_idx ON public.kb_chunks_note USING gin (COALESCE(NULLIF(search_text, ''::text), content) public.gin_trgm_ops);


--
-- Name: kb_chunks_note_content_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_note_content_idx ON public.kb_chunks_note USING gin (content public.gin_trgm_ops);


--
-- Name: kb_chunks_note_embedding_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_note_embedding_idx ON public.kb_chunks_note USING hnsw (embedding public.vector_cosine_ops) WITH (m='16', ef_construction='64') WHERE ((index_ready = true) AND (embedding IS NOT NULL));


--
-- Name: kb_chunks_note_embedding_structured_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_note_embedding_structured_idx ON public.kb_chunks_note USING hnsw (embedding_structured public.vector_cosine_ops) WITH (m='16', ef_construction='64') WHERE ((index_ready = true) AND (embedding_structured IS NOT NULL));


--
-- Name: kb_chunks_note_kb_type_index_ready_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_note_kb_type_index_ready_idx ON public.kb_chunks_note USING btree (kb_type, index_ready) WHERE (index_ready = true);


--
-- Name: kb_chunks_note_search_text_tsv_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_note_search_text_tsv_idx ON public.kb_chunks_note USING gin (search_text_tsv);


--
-- Name: kb_chunks_note_to_tsvector_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_note_to_tsvector_idx ON public.kb_chunks_note USING gin (to_tsvector('simple'::regconfig, content));


--
-- Name: kb_chunks_note_to_tsvector_idx1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_note_to_tsvector_idx1 ON public.kb_chunks_note USING gin (to_tsvector('simple'::regconfig, COALESCE(NULLIF(search_text, ''::text), content)));


--
-- Name: kb_chunks_paper_coalesce_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_paper_coalesce_idx ON public.kb_chunks_paper USING gin (COALESCE(NULLIF(search_text, ''::text), content) public.gin_trgm_ops);


--
-- Name: kb_chunks_paper_content_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_paper_content_idx ON public.kb_chunks_paper USING gin (content public.gin_trgm_ops);


--
-- Name: kb_chunks_paper_embedding_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_paper_embedding_idx ON public.kb_chunks_paper USING hnsw (embedding public.vector_cosine_ops) WITH (m='16', ef_construction='64') WHERE ((index_ready = true) AND (embedding IS NOT NULL));


--
-- Name: kb_chunks_paper_embedding_structured_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_paper_embedding_structured_idx ON public.kb_chunks_paper USING hnsw (embedding_structured public.vector_cosine_ops) WITH (m='16', ef_construction='64') WHERE ((index_ready = true) AND (embedding_structured IS NOT NULL));


--
-- Name: kb_chunks_paper_kb_type_index_ready_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_paper_kb_type_index_ready_idx ON public.kb_chunks_paper USING btree (kb_type, index_ready) WHERE (index_ready = true);


--
-- Name: kb_chunks_paper_search_text_tsv_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_paper_search_text_tsv_idx ON public.kb_chunks_paper USING gin (search_text_tsv);


--
-- Name: kb_chunks_paper_to_tsvector_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_paper_to_tsvector_idx ON public.kb_chunks_paper USING gin (to_tsvector('simple'::regconfig, content));


--
-- Name: kb_chunks_paper_to_tsvector_idx1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_paper_to_tsvector_idx1 ON public.kb_chunks_paper USING gin (to_tsvector('simple'::regconfig, COALESCE(NULLIF(search_text, ''::text), content)));


--
-- Name: kb_chunks_tutorial_coalesce_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_tutorial_coalesce_idx ON public.kb_chunks_tutorial USING gin (COALESCE(NULLIF(search_text, ''::text), content) public.gin_trgm_ops);


--
-- Name: kb_chunks_tutorial_content_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_tutorial_content_idx ON public.kb_chunks_tutorial USING gin (content public.gin_trgm_ops);


--
-- Name: kb_chunks_tutorial_embedding_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_tutorial_embedding_idx ON public.kb_chunks_tutorial USING hnsw (embedding public.vector_cosine_ops) WITH (m='16', ef_construction='64') WHERE ((index_ready = true) AND (embedding IS NOT NULL));


--
-- Name: kb_chunks_tutorial_embedding_structured_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_tutorial_embedding_structured_idx ON public.kb_chunks_tutorial USING hnsw (embedding_structured public.vector_cosine_ops) WITH (m='16', ef_construction='64') WHERE ((index_ready = true) AND (embedding_structured IS NOT NULL));


--
-- Name: kb_chunks_tutorial_kb_type_index_ready_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_tutorial_kb_type_index_ready_idx ON public.kb_chunks_tutorial USING btree (kb_type, index_ready) WHERE (index_ready = true);


--
-- Name: kb_chunks_tutorial_search_text_tsv_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_tutorial_search_text_tsv_idx ON public.kb_chunks_tutorial USING gin (search_text_tsv);


--
-- Name: kb_chunks_tutorial_to_tsvector_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_tutorial_to_tsvector_idx ON public.kb_chunks_tutorial USING gin (to_tsvector('simple'::regconfig, content));


--
-- Name: kb_chunks_tutorial_to_tsvector_idx1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_chunks_tutorial_to_tsvector_idx1 ON public.kb_chunks_tutorial USING gin (to_tsvector('simple'::regconfig, COALESCE(NULLIF(search_text, ''::text), content)));


--
-- Name: kb_qa_pairs_default_doc_id_kb_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX kb_qa_pairs_default_doc_id_kb_type_idx ON public.kb_qa_pairs_default USING btree (doc_id, kb_type);


--
-- Name: kb_chunks_default_coalesce_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_search_text_trgm ATTACH PARTITION public.kb_chunks_default_coalesce_idx;


--
-- Name: kb_chunks_default_content_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_content_trgm ATTACH PARTITION public.kb_chunks_default_content_idx;


--
-- Name: kb_chunks_default_doc_id_chunk_index_kb_type_key; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.kb_chunks_doc_id_chunk_index_kb_type_key ATTACH PARTITION public.kb_chunks_default_doc_id_chunk_index_kb_type_key;


--
-- Name: kb_chunks_default_embedding_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_embedding_hnsw ATTACH PARTITION public.kb_chunks_default_embedding_idx;


--
-- Name: kb_chunks_default_embedding_structured_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_embedding_structured_hnsw ATTACH PARTITION public.kb_chunks_default_embedding_structured_idx;


--
-- Name: kb_chunks_default_kb_type_index_ready_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_ready_kb_type ATTACH PARTITION public.kb_chunks_default_kb_type_index_ready_idx;


--
-- Name: kb_chunks_default_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.kb_chunks_pkey ATTACH PARTITION public.kb_chunks_default_pkey;


--
-- Name: kb_chunks_default_search_text_tsv_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_chunks_search_text_gin ATTACH PARTITION public.kb_chunks_default_search_text_tsv_idx;


--
-- Name: kb_chunks_default_to_tsvector_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_content_fts ATTACH PARTITION public.kb_chunks_default_to_tsvector_idx;


--
-- Name: kb_chunks_default_to_tsvector_idx1; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_search_text_fts ATTACH PARTITION public.kb_chunks_default_to_tsvector_idx1;


--
-- Name: kb_chunks_note_coalesce_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_search_text_trgm ATTACH PARTITION public.kb_chunks_note_coalesce_idx;


--
-- Name: kb_chunks_note_content_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_content_trgm ATTACH PARTITION public.kb_chunks_note_content_idx;


--
-- Name: kb_chunks_note_doc_id_chunk_index_kb_type_key; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.kb_chunks_doc_id_chunk_index_kb_type_key ATTACH PARTITION public.kb_chunks_note_doc_id_chunk_index_kb_type_key;


--
-- Name: kb_chunks_note_embedding_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_embedding_hnsw ATTACH PARTITION public.kb_chunks_note_embedding_idx;


--
-- Name: kb_chunks_note_embedding_structured_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_embedding_structured_hnsw ATTACH PARTITION public.kb_chunks_note_embedding_structured_idx;


--
-- Name: kb_chunks_note_kb_type_index_ready_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_ready_kb_type ATTACH PARTITION public.kb_chunks_note_kb_type_index_ready_idx;


--
-- Name: kb_chunks_note_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.kb_chunks_pkey ATTACH PARTITION public.kb_chunks_note_pkey;


--
-- Name: kb_chunks_note_search_text_tsv_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_chunks_search_text_gin ATTACH PARTITION public.kb_chunks_note_search_text_tsv_idx;


--
-- Name: kb_chunks_note_to_tsvector_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_content_fts ATTACH PARTITION public.kb_chunks_note_to_tsvector_idx;


--
-- Name: kb_chunks_note_to_tsvector_idx1; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_search_text_fts ATTACH PARTITION public.kb_chunks_note_to_tsvector_idx1;


--
-- Name: kb_chunks_paper_coalesce_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_search_text_trgm ATTACH PARTITION public.kb_chunks_paper_coalesce_idx;


--
-- Name: kb_chunks_paper_content_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_content_trgm ATTACH PARTITION public.kb_chunks_paper_content_idx;


--
-- Name: kb_chunks_paper_doc_id_chunk_index_kb_type_key; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.kb_chunks_doc_id_chunk_index_kb_type_key ATTACH PARTITION public.kb_chunks_paper_doc_id_chunk_index_kb_type_key;


--
-- Name: kb_chunks_paper_embedding_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_embedding_hnsw ATTACH PARTITION public.kb_chunks_paper_embedding_idx;


--
-- Name: kb_chunks_paper_embedding_structured_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_embedding_structured_hnsw ATTACH PARTITION public.kb_chunks_paper_embedding_structured_idx;


--
-- Name: kb_chunks_paper_kb_type_index_ready_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_ready_kb_type ATTACH PARTITION public.kb_chunks_paper_kb_type_index_ready_idx;


--
-- Name: kb_chunks_paper_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.kb_chunks_pkey ATTACH PARTITION public.kb_chunks_paper_pkey;


--
-- Name: kb_chunks_paper_search_text_tsv_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_chunks_search_text_gin ATTACH PARTITION public.kb_chunks_paper_search_text_tsv_idx;


--
-- Name: kb_chunks_paper_to_tsvector_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_content_fts ATTACH PARTITION public.kb_chunks_paper_to_tsvector_idx;


--
-- Name: kb_chunks_paper_to_tsvector_idx1; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_search_text_fts ATTACH PARTITION public.kb_chunks_paper_to_tsvector_idx1;


--
-- Name: kb_chunks_tutorial_coalesce_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_search_text_trgm ATTACH PARTITION public.kb_chunks_tutorial_coalesce_idx;


--
-- Name: kb_chunks_tutorial_content_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_content_trgm ATTACH PARTITION public.kb_chunks_tutorial_content_idx;


--
-- Name: kb_chunks_tutorial_doc_id_chunk_index_kb_type_key; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.kb_chunks_doc_id_chunk_index_kb_type_key ATTACH PARTITION public.kb_chunks_tutorial_doc_id_chunk_index_kb_type_key;


--
-- Name: kb_chunks_tutorial_embedding_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_embedding_hnsw ATTACH PARTITION public.kb_chunks_tutorial_embedding_idx;


--
-- Name: kb_chunks_tutorial_embedding_structured_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_embedding_structured_hnsw ATTACH PARTITION public.kb_chunks_tutorial_embedding_structured_idx;


--
-- Name: kb_chunks_tutorial_kb_type_index_ready_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_ready_kb_type ATTACH PARTITION public.kb_chunks_tutorial_kb_type_index_ready_idx;


--
-- Name: kb_chunks_tutorial_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.kb_chunks_pkey ATTACH PARTITION public.kb_chunks_tutorial_pkey;


--
-- Name: kb_chunks_tutorial_search_text_tsv_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_chunks_search_text_gin ATTACH PARTITION public.kb_chunks_tutorial_search_text_tsv_idx;


--
-- Name: kb_chunks_tutorial_to_tsvector_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_content_fts ATTACH PARTITION public.kb_chunks_tutorial_to_tsvector_idx;


--
-- Name: kb_chunks_tutorial_to_tsvector_idx1; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_chunks_search_text_fts ATTACH PARTITION public.kb_chunks_tutorial_to_tsvector_idx1;


--
-- Name: kb_qa_pairs_default_doc_id_kb_type_idx; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.idx_kb_qa_pairs_doc ATTACH PARTITION public.kb_qa_pairs_default_doc_id_kb_type_idx;


--
-- Name: kb_qa_pairs_default_pkey; Type: INDEX ATTACH; Schema: public; Owner: -
--

ALTER INDEX public.kb_qa_pairs_pkey ATTACH PARTITION public.kb_qa_pairs_default_pkey;


--
-- Name: kb_chunks trg_kb_chunks_updated; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_kb_chunks_updated BEFORE UPDATE ON public.kb_chunks FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();


--
-- Name: kb_documents trg_kb_documents_updated; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_kb_documents_updated BEFORE UPDATE ON public.kb_documents FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();


--
-- Name: agent_run_events agent_run_events_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_run_events
    ADD CONSTRAINT agent_run_events_run_id_fkey FOREIGN KEY (run_id) REFERENCES public.agent_runs(run_id) ON DELETE CASCADE;


--
-- Name: kb_catalog_version kb_catalog_version_kb_type_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_catalog_version
    ADD CONSTRAINT kb_catalog_version_kb_type_fkey FOREIGN KEY (kb_type) REFERENCES public.kb_types(name);


--
-- Name: kb_chunks kb_chunks_doc_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.kb_chunks
    ADD CONSTRAINT kb_chunks_doc_id_fkey FOREIGN KEY (doc_id) REFERENCES public.kb_documents(id) ON DELETE CASCADE;


--
-- Name: kb_doc_tags kb_doc_tags_doc_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_doc_tags
    ADD CONSTRAINT kb_doc_tags_doc_id_fkey FOREIGN KEY (doc_id) REFERENCES public.kb_documents(id) ON DELETE CASCADE;


--
-- Name: kb_documents kb_documents_kb_type_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.kb_documents
    ADD CONSTRAINT kb_documents_kb_type_fkey FOREIGN KEY (kb_type) REFERENCES public.kb_types(name);


--
-- Name: kb_qa_pairs kb_qa_pairs_doc_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.kb_qa_pairs
    ADD CONSTRAINT kb_qa_pairs_doc_id_fkey FOREIGN KEY (doc_id) REFERENCES public.kb_documents(id) ON DELETE CASCADE;


--
-- Name: knowledge_bases knowledge_bases_storage_partition_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_bases
    ADD CONSTRAINT knowledge_bases_storage_partition_fkey FOREIGN KEY (storage_partition) REFERENCES public.kb_types(name);


--
-- Name: knowledge_bases knowledge_bases_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_bases
    ADD CONSTRAINT knowledge_bases_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: tenant_tool_bindings tenant_tool_bindings_tool_name_tool_version_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_tool_bindings
    ADD CONSTRAINT tenant_tool_bindings_tool_name_tool_version_fkey FOREIGN KEY (tool_name, tool_version) REFERENCES public.tool_specs(name, version);


--
-- Name: ugc_ingest_tasks ugc_ingest_tasks_kb_type_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ugc_ingest_tasks
    ADD CONSTRAINT ugc_ingest_tasks_kb_type_fkey FOREIGN KEY (kb_type) REFERENCES public.kb_types(name);


--
-- Name: workspace_4d_taxonomy workspace_4d_taxonomy_kb_type_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workspace_4d_taxonomy
    ADD CONSTRAINT workspace_4d_taxonomy_kb_type_fkey FOREIGN KEY (kb_type) REFERENCES public.kb_types(name) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--
