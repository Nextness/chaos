$$
\begin{align}
    \text{program} &\to [\text{statement}]^* \\
    \text{statement} &\to 
        \begin{cases} 
            % TODO: Update exit to have a message as well...
            \text{exit} \space [\text{expression}]; \\
            \text{let} \space \text{identifier} = \text{expression};
        \end{cases} \\
    \text{expression} &\to 
        \begin{cases}
            integerLiteral \\
            \text{identifier} \\
        \end{cases} \\
\end{align}
$$