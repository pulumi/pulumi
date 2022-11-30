export type PType =
  | PObject
  | string
  | number
  | boolean
  | Map<string, PType>
  | null;

export type PObject = {
  __name: string;
  __module: string;
};
