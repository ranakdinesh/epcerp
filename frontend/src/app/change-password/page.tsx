import { redirect } from "next/navigation";
import { getAuthenticatedUser } from "@/lib/auth/session";
import FirstPasswordChange from "@/components/auth/first-password-change";
export const metadata = { title: "Choose your password | epcERP", robots: {index:false,follow:false}, referrer:"no-referrer" as const };
export default async function ChangePasswordPage(){
 const user=await getAuthenticatedUser();
 if(!user)redirect("/login?session=expired");
 if(!user.must_change_password)redirect("/dashboard");
 return <FirstPasswordChange/>;
}
