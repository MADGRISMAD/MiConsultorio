-- Blog de Caresia: artículos escritos con IA (o a mano) que se publican en /blog/{slug}.
CREATE TABLE blog_posts (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            text NOT NULL UNIQUE,
    title           text NOT NULL,
    description     text NOT NULL DEFAULT '',   -- meta description para Google
    excerpt         text NOT NULL DEFAULT '',   -- resumen para la lista del blog
    keyword         text NOT NULL DEFAULT '',
    category        text NOT NULL DEFAULT '',
    topic           text NOT NULL DEFAULT '',   -- clave del banco de temas; vacío si el tema lo eligió la IA o una persona
    body            jsonb NOT NULL DEFAULT '[]', -- bloques: [{type: h2|h3|p|ul|ol|tip, text, items}]
    faqs            jsonb NOT NULL DEFAULT '[]', -- [{q, a}]
    reading_minutes int NOT NULL DEFAULT 0,
    status          text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    published_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX blog_posts_published_idx ON blog_posts (published_at DESC) WHERE status = 'published';

-- Una corrida automática por día programado: evita que dos servidores escriban el mismo artículo
-- y limita los reintentos cuando la IA falla.
CREATE TABLE blog_runs (
    day        date PRIMARY KEY,
    attempts   int NOT NULL DEFAULT 1,
    done       boolean NOT NULL DEFAULT false,
    started_at timestamptz NOT NULL DEFAULT now()
);
