import fullFold from '../../master/catalogue/full-fold.json';
import { trim } from './validation';
const folds=new Map<number,string>((fullFold as [number,string][]).map(([key,value])=>[key,value]));
export const nameKey=(value:string)=>[...trim(value)].map(c=>folds.get(c.codePointAt(0)!)??c).join('');
