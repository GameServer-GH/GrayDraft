CREATE TABLE IF NOT EXISTS public.product_threads (
    tenant_id text NOT NULL,
    actor_id text NOT NULL,
    thread_key text NOT NULL,
    payload jsonb NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT product_threads_pkey PRIMARY KEY (tenant_id, actor_id, thread_key)
);
