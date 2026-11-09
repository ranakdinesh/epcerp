export type CompanyProfile = {
  id: string;
  legal_name: string;
  trade_name: string;
  cin: string;
  pan: string;
  gstin: string;
  incorporation_date: string | null;
  registered_address: string;
  registered_state: string;
  registered_city: string;
  registered_pincode: string;
  epfo_registration: string;
  esic_registration: string;
  msme_udyam_number: string;
  msme_category: 'micro' | 'small' | 'medium' | '';
  msme_activity: 'manufacturing' | 'services' | 'trading' | '';
  startup_recognition_number: string;
  iso_9001: boolean;
  iso_14001: boolean;
  iso_45001: boolean;
  non_blacklisted_declaration: boolean;
  clean_insolvency_declaration: boolean;
  version: number;
  updated_at: string;
};

export type CompanyFinancialYear = {
  id: string;
  financial_year: string;
  turnover_excluding_gst: number;
  net_worth: number;
  profit_after_tax: number;
  is_audited: boolean;
  version: number;
  updated_at: string;
};

export type CompanyCertificate = {
  id: string;
  kind: 'solvency' | 'working_capital' | 'bank_guarantee_limit' | 'iso_certificate';
  certificate_number: string;
  issuer_name: string;
  amount: number;
  issued_on: string | null;
  valid_until: string | null;
  is_active: boolean;
  version: number;
  updated_at: string;
};

export type CompanyLicence = {
  id: string;
  licence_number: string;
  state: string;
  class_grade: string;
  maximum_voltage_kv: number;
  issued_on: string | null;
  valid_until: string;
  is_active: boolean;
  version: number;
  updated_at: string;
};

export type CompanyWorkExperience = {
  id: string;
  project_title: string;
  client_name: string;
  client_type: 'central_govt' | 'state_govt' | 'psu' | 'public_ltd' | 'private';
  scope_description: string;
  executed_value_excluding_gst: number;
  state: string;
  voltage_kv: number | null;
  capacity_description: string;
  company_share_percent: number;
  completed_on: string;
  completion_certificate_ref: string;
  is_active: boolean;
  version: number;
  updated_at: string;
};

export type FullCompanySetup = {
  profile: CompanyProfile;
  financials: CompanyFinancialYear[];
  certificates: CompanyCertificate[];
  licences: CompanyLicence[];
  work_experience: CompanyWorkExperience[];
};
