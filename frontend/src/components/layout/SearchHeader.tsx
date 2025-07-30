import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { SidebarTrigger } from "@/components/ui/sidebar";

export function SearchHeader() {
  const [searchQuery, setSearchQuery] = useState("");
  const navigate = useNavigate();

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (searchQuery.trim()) {
      navigate(`/discover?query=${encodeURIComponent(searchQuery.trim())}`);
    }
  };

  return (
    <header className="flex items-center gap-4 px-6 py-4 border-b border-border bg-card">
      <SidebarTrigger className="md:hidden" />
      
      <div className="flex-1 max-w-md">
        <form onSubmit={handleSearch} className="flex gap-2">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="text"
              placeholder="Search movies and TV shows..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-10 bg-input border-border focus:ring-primary"
            />
          </div>
          <Button type="submit" size="sm" className="bg-gradient-primary hover:opacity-90">
            Search
          </Button>
        </form>
      </div>

      <div className="flex items-center gap-2">
        {/* Future: User avatar and menu */}
      </div>
    </header>
  );
}