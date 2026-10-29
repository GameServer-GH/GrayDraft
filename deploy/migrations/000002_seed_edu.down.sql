DELETE FROM public.kb_documents
 WHERE id IN (
   'a0000000-0000-4000-8000-000000000001',
   'a0000000-0000-4000-8000-000000000002'
 );
DELETE FROM public.kb_catalog_version WHERE kb_type IN ('tutorial', 'paper', 'note');
DELETE FROM public.kb_types WHERE name IN ('tutorial', 'paper', 'note');
