"use client";

import { useState } from "react";
import { Download, FileSpreadsheet, FileText, ChevronDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

interface ExportButtonProps {
  onExportCSV: () => void;
  onExportPDF: () => void;
  isLoading?: boolean;
}

export function ExportButton({ onExportCSV, onExportPDF, isLoading }: ExportButtonProps) {
  const [open, setOpen] = useState(false);

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" disabled={isLoading}>
          <Download className="mr-1.5 h-4 w-4" />
          Export
          <ChevronDown className="ml-1 h-3 w-3" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem
          onClick={() => {
            onExportCSV();
            setOpen(false);
          }}
        >
          <FileSpreadsheet className="mr-2 h-4 w-4" />
          Export as CSV
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => {
            onExportPDF();
            setOpen(false);
          }}
        >
          <FileText className="mr-2 h-4 w-4" />
          Export as PDF
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

// Simple CSV-only export button
export function CSVExportButton({
  onClick,
  isLoading,
  label = "Export CSV",
}: {
  onClick: () => void;
  isLoading?: boolean;
  label?: string;
}) {
  return (
    <Button variant="outline" size="sm" onClick={onClick} disabled={isLoading}>
      <FileSpreadsheet className="mr-1.5 h-4 w-4" />
      {label}
    </Button>
  );
}
