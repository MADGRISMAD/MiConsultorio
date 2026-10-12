-- Todos los consultorios con suscripción vigente aparecen en el directorio; los que completan su perfil suben.
-- Quien no quiera salir lo oculta (antes había que pedir salir: la columna «listed» ya no se usa).
ALTER TABLE clinic_profile ADD COLUMN directory_hidden boolean NOT NULL DEFAULT false;
