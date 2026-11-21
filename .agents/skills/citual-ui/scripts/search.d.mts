export type SearchOptions = {kind?:string;maturity?:string;generic?:boolean;capabilities?:string[];limit?:number};
export declare function searchCatalog<T>(catalog:{items:T[]},query?:string,options?:SearchOptions):(T & {score:number;matchedTerms:string[];reason:string})[];
export declare function main(args?:string[]):Promise<void>;
