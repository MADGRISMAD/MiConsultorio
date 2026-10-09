/** The owner (propietario) of animal patients: a record of the clinic that its pets point to. */
export interface OwnerRef {
  id: string;
  name: string;
  phone: string;
  email: string;
  /** informative: the owner's domicilio and birth date */
  address?: string;
  birth_date?: string;
}

export interface OwnerPet {
  id: string;
  file_number: number;
  names: string;
  last_names: string;
  species: string;
  archived: boolean;
  /** the search found this pet itself (not only through its owner) */
  matched: boolean;
  last_visit?: string | null;
}

export interface OwnerGroup {
  owner: OwnerRef;
  pets: OwnerPet[];
}

export interface OwnerListItem extends OwnerRef {
  pet_count: number;
  pets: OwnerPet[];
}
