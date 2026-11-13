import { redirect } from "next/navigation";

// games moved to /play; old links (notifications, history) still land on the board
export default function OldPlayPage({ params }: { params: { gameId: string } }) {
  redirect(`/play/${params.gameId}`);
}
