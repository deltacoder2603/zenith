'use client';

import { cn } from "@/lib/utils";
import React, { useState } from "react";
import Navbar from "@/components/Navbar";
import axios from "axios";

interface DeploymentResponse {
  repo: string;
  public_url: string;
  buildResult: any;
}

export default function Home() {
  const [repoUrl, setRepoUrl] = useState<string>("");
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [deploymentResult, setDeploymentResult] = useState<DeploymentResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleDeploy = async (): Promise<void> => {
    if (!repoUrl) {
      setError("Please enter a GitHub repository URL");
      return;
    }

    setIsLoading(true);
    setError(null);
    setDeploymentResult(null);

    try {
      const response = await axios.post("http://localhost:8080/deploy", {
        url: repoUrl,
      });

      const { repo, public_url, buildResult } = response.data;

      setDeploymentResult({ repo, public_url, buildResult });
    } catch (err: any) {
      console.error("Deployment error:", err);
      setError(err.response?.data?.error || "Failed to deploy. Please try again.");
    } finally {
      setIsLoading(false);
    }
  };

  const copyToClipboard = (text: string): void => {
    navigator.clipboard.writeText(text)
      .catch(err => console.error('Failed to copy text: ', err));
  };

  return (
    <>
      <div className="relative flex flex-col w-full min-h-screen bg-white dark:bg-black overflow-auto">
        {/* Corner blurs */}
        <div className="absolute top-0 left-0 w-64 h-64 bg-blue-400 dark:bg-blue-600 rounded-full opacity-20 blur-3xl" />
        <div className="absolute bottom-0 right-0 w-64 h-64 bg-purple-400 dark:bg-purple-600 rounded-full opacity-20 blur-3xl" />
        <div className="absolute bottom-0 left-0 w-48 h-48 bg-green-400 dark:bg-green-600 rounded-full opacity-10 blur-3xl" />
        <div className="absolute top-0 right-0 w-48 h-48 bg-pink-400 dark:bg-pink-600 rounded-full opacity-10 blur-3xl" />

        {/* Grid background */}
        <div
          className={cn(
            "absolute inset-0",
            "[background-size:40px_40px]",
            "[background-image:linear-gradient(to_right,#e4e4e7_1px,transparent_1px),linear-gradient(to_bottom,#e4e4e7_1px,transparent_1px)]",
            "dark:[background-image:linear-gradient(to_right,#262626_1px,transparent_1px),linear-gradient(to_bottom,#262626_1px,transparent_1px)]",
            "opacity-50"
          )}
        />

        <Navbar />

        {/* Responsive layout */}
        <div className="relative z-10 flex flex-col md:flex-row w-full h-full flex-1">
          {/* Left - Hero Section */}
          <div className="w-full md:w-1/2 flex items-center justify-center px-4 py-12">
            <div className="flex flex-col items-center text-center max-w-xl">
              <h1 className="text-4xl sm:text-5xl md:text-6xl font-bold leading-tight text-transparent bg-clip-text bg-gradient-to-r from-black to-gray-700 dark:from-white dark:to-gray-400 tracking-tight mb-6">
                Deployments<br /> made easy
              </h1>
              <p className="text-base sm:text-lg md:text-xl text-gray-600 dark:text-gray-400 max-w-md mb-8 leading-relaxed">
                Instantly deploy your sites to production with one click, no configuration required.
              </p>
            </div>
          </div>

          {/* Right - Form */}
          <div className="w-full md:w-1/2 flex items-center justify-center px-4 py-12">
            <div className="w-full max-w-md backdrop-blur-lg bg-white/30 dark:bg-gray-800/30 rounded-2xl shadow-xl p-6 sm:p-8 border border-white/20 dark:border-gray-700/30">
              <div className="mb-8">
                <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Deploy your project</h2>
                <p className="text-sm text-gray-600 dark:text-gray-300 mt-2">Enter your GitHub repository URL to deploy in seconds</p>
              </div>

              <div className="space-y-6">
                <div className="space-y-2">
                  <label htmlFor="github-url" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    GitHub Repository URL
                  </label>
                  <div className="relative">
                    <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                      <svg className="h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"></path>
                      </svg>
                    </div>
                    <input
                      id="github-url"
                      type="text"
                      className="w-full pl-10 pr-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white/80 dark:bg-gray-700/80 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 focus:border-transparent transition duration-200"
                      placeholder="https://github.com/username/repo"
                      value={repoUrl}
                      onChange={(e: React.ChangeEvent<HTMLInputElement>) => setRepoUrl(e.target.value)}
                    />
                  </div>
                  {error && (
                    <p className="text-sm text-red-600 dark:text-red-400 mt-1">{error}</p>
                  )}
                </div>

                <button
                  onClick={handleDeploy}
                  disabled={isLoading}
                  className="w-full px-4 py-3 bg-gradient-to-r from-blue-500 to-purple-600 dark:from-blue-600 dark:to-purple-700 text-white font-medium rounded-lg shadow-md hover:shadow-lg transform hover:translate-y-0.5 transition-all duration-200 disabled:opacity-70 disabled:cursor-not-allowed flex items-center justify-center"
                >
                  {isLoading ? (
                    <>
                      <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                      </svg>
                      Deploying...
                    </>
                  ) : (
                    <>
                      Deploy Project
                      <svg className="ml-2 h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 5l7 7-7 7M5 5l7 7-7 7" />
                      </svg>
                    </>
                  )}
                </button>
              </div>

              {deploymentResult && (
                <div className="mt-8 p-5 border border-green-200 dark:border-green-800 bg-green-50/50 dark:bg-green-900/20 backdrop-blur-sm rounded-lg">
                  <div className="flex items-center mb-3">
                    <svg className="h-5 w-5 text-green-500 mr-2" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7" />
                    </svg>
                    <h3 className="text-lg font-medium text-gray-900 dark:text-white">Deployment Successful</h3>
                  </div>

                  <div className="space-y-4">
                    <div>
                      <p className="text-sm text-gray-500 dark:text-gray-400">Repository</p>
                      <p className="text-sm font-medium text-gray-900 dark:text-white">{deploymentResult.repo}</p>
                    </div>

                    <div>
                      <p className="text-sm text-gray-500 dark:text-gray-400">Deployment URL</p>
                      <div className="flex items-center mt-1">
                        <input
                          type="text"
                          className="flex-1 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-l-lg bg-white/80 dark:bg-gray-700/80 text-gray-900 dark:text-gray-100"
                          readOnly
                          value={deploymentResult.public_url}
                        />
                        <button
                          className="px-3 py-2 bg-gray-100 dark:bg-gray-600 border border-gray-300 dark:border-gray-600 border-l-0 rounded-r-lg text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-500"
                          onClick={() => copyToClipboard(deploymentResult.public_url)}
                        >
                          <svg className="h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3" />
                          </svg>
                        </button>
                      </div>
                    </div>

                    <a
                      href={deploymentResult.public_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="block w-full px-4 py-2 bg-blue-500 dark:bg-blue-600 text-center text-white font-medium rounded-lg hover:bg-blue-600 dark:hover:bg-blue-700 transition-colors"
                    >
                      Visit Website
                    </a>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </>
  );
}