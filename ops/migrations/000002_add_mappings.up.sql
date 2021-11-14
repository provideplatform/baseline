--
-- Name: mappings; Type: TABLE; Schema: public; Owner: baseline
--

CREATE TABLE public.mappings (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    created_at timestamp with time zone NOT NULL,
    name text NOT NULL,
    description text,
    type varchar(64),
    organization_id uuid NOT NULL,
    workgroup_id uuid NOT NULL
);

ALTER TABLE public.mappings OWNER TO baseline;

ALTER TABLE ONLY public.mappings
    ADD CONSTRAINT mappings_pkey PRIMARY KEY (id);

CREATE INDEX idx_mappings_type ON public.mappings USING btree (type);
CREATE INDEX idx_mappings_organization_id_workgroup_id ON public.mappings USING btree (organization_id, workgroup_id);

ALTER TABLE ONLY public.mappings
  ADD CONSTRAINT mappings_workgroup_id_foreign FOREIGN KEY (workgroup_id) REFERENCES public.workgroups(id) ON UPDATE CASCADE ON DELETE CASCADE;

CREATE TABLE public.mappingmodels (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    created_at timestamp with time zone NOT NULL,
    mapping_id uuid NOT NULL,
    type text NOT NULL,
    primary_key text NOT NULL,
    description text
);

ALTER TABLE public.mappingmodels OWNER TO baseline;

ALTER TABLE ONLY public.mappingmodels
    ADD CONSTRAINT mappingmodels_pkey PRIMARY KEY (id);

CREATE INDEX idx_mappingmodels_type ON public.mappingmodels USING btree (type);
CREATE INDEX idx_mappingmodels_mapping_id ON public.mappingmodels USING btree (mapping_id);

ALTER TABLE ONLY public.mappingmodels
  ADD CONSTRAINT mappingmodels_mapping_id_foreign FOREIGN KEY (mapping_id) REFERENCES public.mappings(id) ON UPDATE CASCADE ON DELETE CASCADE;

CREATE TABLE public.mappingfields (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    created_at timestamp with time zone NOT NULL,
    mappingmodel_id uuid NOT NULL,
    name text NOT NULL,
    description text,
    default_value varchar(64),
    is_primary_key bool NOT NULL DEFAULT false
);

ALTER TABLE public.mappingfields OWNER TO baseline;

ALTER TABLE ONLY public.mappingfields
    ADD CONSTRAINT mappingfields_pkey PRIMARY KEY (id);

CREATE INDEX idx_mappingfields_mappingmodel_id ON public.mappingfields USING btree (mappingmodel_id);

ALTER TABLE ONLY public.mappingfields
  ADD CONSTRAINT mappingfields_mappingmodel_id_foreign FOREIGN KEY (mappingmodel_id) REFERENCES public.mappingmodels(id) ON UPDATE CASCADE ON DELETE CASCADE;