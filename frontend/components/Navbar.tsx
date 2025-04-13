"use client";

import { Linkedin, Github, Instagram, Twitter } from "lucide-react";

export default function Navbar() {
  return (
    <div>
      <header className="fixed top-0 left-0 right-0 z-50 flex items-center justify-between bg-black rounded-xl p-2 m-3 px-6 py-4 border-b border-b-white h-16 shadow-lg">
        
        {/* Logo */}
        <a href="/">
          <div className="flex items-center space-x-3">
            <img src="./logo.png" className="h-10 w-10" alt="Logo" />
            <h1 className="text-2xl font-bold text-white">Zenith</h1>
          </div>
        </a>

        {/* Social Icons */}
        <div className="flex items-center space-x-4">
          <a href="https://www.linkedin.com/in/srikant-pandey-b55935209/" target="_blank" rel="noopener noreferrer" className="text-white hover:text-blue-400 transition-colors">
            <Linkedin className="h-5 w-5" />
          </a>
          <a href="https://github.com/deltacoder2603" target="_blank" rel="noopener noreferrer" className="text-white hover:text-gray-300 transition-colors">
            <Github className="h-5 w-5" />
          </a>
          <a href="https://www.instagram.com/_a_tesorino_/" target="_blank" rel="noopener noreferrer" className="text-white hover:text-pink-400 transition-colors">
            <Instagram className="h-5 w-5" />
          </a>
          <a href="https://x.com/DeltaPandey2603" target="_blank" rel="noopener noreferrer" className="text-white hover:text-blue-300 transition-colors">
            <Twitter className="h-5 w-5" />
          </a>
        </div>
      </header>

      {/* Spacer div so content isn't hidden behind navbar */}
      <div className="h-16" />
    </div>
  );
}